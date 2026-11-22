package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

type completeReplacement struct {
	oldName, oldProtocol, newName, newProtocol string
}

var completeReplacements = []completeReplacement{
	{"wireguard", "wireguard", "managed-ss2022-aes128", "ss2022-aes128"},
	{"managed-tuic", "tuic", "managed-ss-aead-chacha20", "ss-aead-chacha20"},
}

type replacementRecord struct {
	Version      int    `json:"version"`
	OldName      string `json:"old_name"`
	NewName      string `json:"new_name"`
	OldProtocol  string `json:"old_protocol"`
	NewProtocol  string `json:"new_protocol"`
	Port         int    `json:"port"`
	Host         string `json:"host"`
	WasActive    bool   `json:"was_active"`
	Backup       string `json:"backup"`
	Stage        string `json:"stage"`
	OldDirectory string `json:"old_directory"`
	OldOptions   string `json:"old_options"`
	OldIdentity  string `json:"old_identity"`
	NewDirectory string `json:"new_directory,omitempty"`
	NewIdentity  string `json:"new_identity,omitempty"`
}

type replacementOps struct {
	stop, start, confirm, remove func(string) error
	status                       func(string) (string, error)
	install                      func(string, coreplan.Options, []config.UserConfig) (string, error)
	backup                       func(string, string) error
	save                         func(string, replacementRecord) error // optional durability seam
}

func realReplacementOps() replacementOps {
	return replacementOps{stop: coreinstall.Stop, start: coreinstall.Start,
		confirm: coreinstall.Confirm, remove: coreinstall.Remove,
		status:  func(name string) (string, error) { return service.Status(coreinstall.ServiceName(name)) },
		install: coreinstall.InstallForUsers, backup: backupReplacementCore}
}

// Only these old automatic names are retired. Custom-named cores, unrelated
// accounts and the quota database are never restored from this private backup.
func migrateCompleteReplacements(cfg *config.Config, out actions.OutputWriter) (map[string]bool, error) {
	return migrateCompleteReplacementsWith(cfg, out, realReplacementOps())
}

func migrateCompleteReplacementsWith(cfg *config.Config, out actions.OutputWriter, ops replacementOps) (map[string]bool, error) {
	blocked := map[string]bool{}
	eligible := false
	for _, u := range cfg.Users {
		if !u.MeteredOnly() && !u.Expired(time.Now()) {
			eligible = true
		}
	}
	if !eligible {
		return blocked, nil
	}
	for _, spec := range completeReplacements {
		skip, err := replaceCompleteCore(cfg, out, spec, ops)
		if skip {
			blocked[spec.newProtocol] = true
		}
		if err != nil {
			return blocked, err
		}
	}
	return blocked, nil
}

func replacementRead(path string, dest any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return fmt.Errorf("invalid private replacement file: %s", filepath.Base(path))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func replacementOptions(name string) (coreplan.Options, coreplan.State, error) {
	var o coreplan.Options
	var s coreplan.State
	if err := replacementRead(filepath.Join(coreinstall.Root, name, "options.json"), &o); err != nil {
		return o, s, err
	}
	err := replacementRead(filepath.Join(coreinstall.Root, name, "state.json"), &s)
	return o, s, err
}

func replacementHash(v any) string {
	raw, _ := json.Marshal(v)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// Lifecycle flags and per-account address allocations may change independently;
// server keys and the credential seed must not change during a replacement.
func replacementIdentity(st coreplan.State) string {
	return replacementHash([]string{st.Seed, st.ServerPrivate, st.ShortID, st.Path})
}

func replacementDirectory(path string, private bool) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || (private && (info.Mode().Perm()&0077 != 0 || st.Uid != uint32(os.Geteuid()))) {
		return "", fmt.Errorf("invalid private replacement directory: %s", filepath.Base(path))
	}
	return fmt.Sprintf("%d:%d", st.Dev, st.Ino), nil
}

func writeReplacementRecord(path string, r replacementRecord) error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return safefile.Write(path, append(raw, '\n'), 0600)
}

func replacementTargetOptions(r replacementRecord) coreplan.Options {
	o := coreplan.Options{Protocol: r.NewProtocol, Host: r.Host, Port: r.Port, DedicatedXray: true}
	_ = coreplan.Validate(&o) // records are validated before any lifecycle operation
	return o
}

func replaceCompleteCore(cfg *config.Config, out actions.OutputWriter, spec completeReplacement, ops replacementOps) (bool, error) {
	root := filepath.Join(filepath.Dir(coreinstall.Root), "core-replacements")
	journal := filepath.Join(root, spec.oldName+".json")
	oldPath, newPath := filepath.Join(coreinstall.Root, spec.oldName), filepath.Join(coreinstall.Root, spec.newName)
	save := ops.save
	if save == nil {
		save = writeReplacementRecord
	}
	var r replacementRecord
	if _, err := os.Lstat(root); err == nil {
		if _, err = replacementDirectory(root, true); err != nil {
			return false, err
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	err := replacementRead(journal, &r)
	if err == nil {
		o := replacementTargetOptions(r)
		if r.Version != 1 || r.OldName != spec.oldName || r.NewName != spec.newName ||
			r.OldProtocol != spec.oldProtocol || r.NewProtocol != spec.newProtocol || coreplan.Validate(&o) != nil ||
			r.OldDirectory == "" || len(r.OldIdentity) != 64 || len(r.OldOptions) != 64 ||
			filepath.Dir(r.Backup) != root || !strings.HasPrefix(filepath.Base(r.Backup), spec.oldName+"-") {
			return false, fmt.Errorf("invalid private core replacement record for %s", spec.oldName)
		}
		if r.Stage == "done" {
			return false, nil
		}
		switch r.Stage {
		case "prepared", "stopping", "stopped", "installed", "retiring", "rolling-back":
		default:
			return false, fmt.Errorf("unknown replacement stage for %s", spec.oldName)
		}
		if _, err = replacementDirectory(r.Backup, true); err != nil {
			return false, fmt.Errorf("private replacement backup is unavailable: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if r.Stage == "" {
		oldID, err := replacementDirectory(oldPath, false)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		old, err := coreinstall.ReadMeta(spec.oldName)
		if err != nil {
			return false, err
		}
		o, st, err := replacementOptions(spec.oldName)
		if err != nil {
			return false, fmt.Errorf("old default %s is not an account-managed core: %w", spec.oldName, err)
		}
		if old.Protocol != spec.oldProtocol || o.Protocol != spec.oldProtocol || old.Port != o.Port || old.Proto != coreinstall.Proto(spec.oldProtocol) {
			return false, fmt.Errorf("refusing to replace differently configured core %s", spec.oldName)
		}
		if st.Disabled || st.Paused {
			out.Info("Automatic replacement remains off because " + spec.oldName + " was stopped")
			return true, nil
		}
		if _, err = os.Lstat(newPath); !os.IsNotExist(err) {
			return false, fmt.Errorf("replacement name %s already exists; refusing to adopt it", spec.newName)
		}
		active, err := ops.status(spec.oldName)
		// systemctl is-active returns exit status 3 for these known states.
		if err != nil && active != "inactive" && active != "failed" {
			return false, err
		}
		if active != "active" && active != "inactive" && active != "failed" {
			return false, fmt.Errorf("old core %s is changing state", spec.oldName)
		}
		r = replacementRecord{Version: 1, OldName: spec.oldName, NewName: spec.newName,
			OldProtocol: spec.oldProtocol, NewProtocol: spec.newProtocol, Port: old.Port,
			Host: o.Host, WasActive: active == "active", Stage: "prepared",
			OldDirectory: oldID, OldOptions: replacementHash(o), OldIdentity: replacementIdentity(st)}
		target := replacementTargetOptions(r)
		if err = coreplan.Validate(&target); err != nil {
			return false, err
		}
		if err = os.MkdirAll(root, 0700); err != nil {
			return false, err
		}
		if _, err = replacementDirectory(root, true); err != nil {
			return false, err
		}
		if err = safefile.SyncDir(filepath.Dir(root)); err != nil {
			return false, err
		}
		r.Backup, err = os.MkdirTemp(root, spec.oldName+"-")
		if err != nil {
			return false, err
		}
		if err = ops.backup(oldPath, r.Backup); err != nil {
			return false, err
		}
		if err = save(journal, r); err != nil {
			return false, err
		}
		out.Info("Private core replacement backup: " + r.Backup)
	}

	validateOld := func() (coreplan.State, error) {
		o, st, err := replacementOptions(spec.oldName)
		id, dirErr := replacementDirectory(oldPath, false)
		m, metaErr := coreinstall.ReadMeta(spec.oldName)
		if err != nil || dirErr != nil || metaErr != nil || id != r.OldDirectory ||
			m.Protocol != spec.oldProtocol || m.Port != r.Port || m.Proto != coreinstall.Proto(spec.oldProtocol) ||
			replacementHash(o) != r.OldOptions || replacementIdentity(st) != r.OldIdentity {
			return st, fmt.Errorf("old core %s changed during replacement; inspect the private record", spec.oldName)
		}
		return st, nil
	}
	validateNew := func() (coreplan.State, error) {
		o, st, err := replacementOptions(spec.newName)
		id, dirErr := replacementDirectory(newPath, false)
		m, metaErr := coreinstall.ReadMeta(spec.newName)
		if err != nil || dirErr != nil || metaErr != nil || m.Protocol != spec.newProtocol || m.Port != r.Port ||
			m.Proto != coreinstall.Proto(spec.newProtocol) || replacementHash(o) != replacementHash(replacementTargetOptions(r)) ||
			(r.NewDirectory != "" && id != r.NewDirectory) || (r.NewIdentity != "" && replacementIdentity(st) != r.NewIdentity) {
			return st, fmt.Errorf("replacement core %s does not match its private record", spec.newName)
		}
		return st, nil
	}
	rollback := func(cause error) error {
		r.Stage = "rolling-back"
		journalErr := save(journal, r)
		// Never restart the old listener while a failed replacement may still
		// own its port. The journal retains cleanup intent if removal fails.
		if err := removeReplacementCore(spec.newName, spec.newProtocol, r.Port, r.NewDirectory, ops); err != nil {
			return errors.Join(cause, journalErr, fmt.Errorf("replacement cleanup pending: %w", err))
		}
		if _, err := validateOld(); err != nil {
			return errors.Join(cause, journalErr, err)
		}
		if err := restoreReplacementOld(spec.oldName, r.WasActive, ops); err != nil {
			return errors.Join(cause, journalErr, fmt.Errorf("old core recovery pending: %w", err))
		}
		r.Stage, r.NewDirectory, r.NewIdentity = "prepared", "", ""
		return errors.Join(cause, journalErr, save(journal, r))
	}
	if r.Stage == "rolling-back" {
		if err = rollback(nil); err != nil {
			return false, err
		}
	}
	if r.Stage == "prepared" {
		st, err := validateOld()
		if err != nil {
			return false, err
		}
		if st.Disabled || st.Paused {
			return true, nil // an operator stopped it before our stop intent
		}
		if _, err = os.Lstat(newPath); !os.IsNotExist(err) {
			return false, fmt.Errorf("replacement name %s appeared before installation", spec.newName)
		}
		r.Stage = "stopping"
		if err = save(journal, r); err != nil {
			return false, err
		}
	}
	if r.Stage == "stopping" {
		if _, err = validateOld(); err != nil {
			return false, err
		}
		if err = ops.stop(spec.oldName); err != nil {
			return false, rollback(err)
		}
		r.Stage = "stopped"
		if err = save(journal, r); err != nil {
			return false, rollback(err)
		}
	}
	ready := false
	if r.Stage == "stopped" {
		if _, err = validateOld(); err != nil {
			return false, err
		}
		// A previous rollback may have restored the old service before the
		// journal write failed. Re-stop it before reusing the same UDP port.
		if err = ops.stop(spec.oldName); err != nil {
			return false, rollback(err)
		}
		_, statErr := os.Lstat(newPath)
		fresh := os.IsNotExist(statErr)
		if statErr != nil && !fresh {
			return false, rollback(statErr)
		}
		if fresh {
			_, err = ops.install(spec.newName, replacementTargetOptions(r), cfg.Users)
			// Bind the directory even after an installer cleanup failure, so a
			// partial directory can be retired without touching a later namesake.
			r.NewDirectory, _ = replacementDirectory(newPath, false)
			if err != nil {
				return false, rollback(fmt.Errorf("replacement %s failed: %w", spec.newName, err))
			}
		}
		st, err := validateNew()
		if err != nil {
			if fresh {
				return false, rollback(err)
			}
			return false, err // never clean up an unrecognized crash-time namesake
		}
		r.NewDirectory, err = replacementDirectory(newPath, false)
		if err != nil {
			return false, err
		}
		r.NewIdentity = replacementIdentity(st)
		if err = save(journal, r); err != nil {
			return false, err
		}
		if st.Disabled || st.Paused {
			return false, fmt.Errorf("replacement %s was stopped; leaving it off", spec.newName)
		}
		if !fresh {
			// Finish an interrupted installer using its existing files and keys.
			if status, _ := ops.status(spec.newName); status != "active" {
				if err = ops.start(spec.newName); err != nil {
					return false, rollback(err)
				}
			}
		}
		if err = ops.confirm(spec.newName); err != nil {
			return false, rollback(err)
		}
		ready = true
		r.Stage = "installed"
		if err = save(journal, r); err != nil {
			return false, err // the ready replacement is reused on the next run
		}
	}
	if !ready {
		st, err := validateNew()
		if err != nil {
			return false, err
		}
		if st.Disabled || st.Paused {
			return false, fmt.Errorf("replacement %s was stopped; leaving it off", spec.newName)
		}
		if status, _ := ops.status(spec.newName); status != "active" {
			if err = ops.start(spec.newName); err != nil {
				return false, err
			}
		}
		if err = ops.confirm(spec.newName); err != nil {
			return false, err
		}
	}
	if r.Stage == "installed" {
		if _, err = validateOld(); err != nil {
			return false, err
		}
		r.Stage = "retiring"
		if err = save(journal, r); err != nil {
			return false, err
		}
	}
	if err = removeReplacementCore(spec.oldName, spec.oldProtocol, r.Port, r.OldDirectory, ops); err != nil {
		return false, fmt.Errorf("replacement is ready; retry retiring %s: %w", spec.oldName, err)
	}
	r.Stage = "done"
	if err = save(journal, r); err != nil {
		return false, err
	}
	out.Info(fmt.Sprintf("Replaced %s with %s on the existing port %d/tcp,udp", spec.oldName, spec.newProtocol, r.Port))
	return false, nil
}

// Remove can fail after deleting only some directory members. The recorded
// directory identity allows retrying that cleanup, including recreating only
// missing metadata needed by Remove; no account, seed or server file is restored.
func removeReplacementCore(name, protocol string, port int, directory string, ops replacementOps) error {
	path := filepath.Join(coreinstall.Root, name)
	id, err := replacementDirectory(path, false)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || directory == "" || id != directory {
		return fmt.Errorf("refusing to remove unrecognized core %s", name)
	}
	metaPath := filepath.Join(path, "meta")
	if info, err := os.Lstat(metaPath); os.IsNotExist(err) {
		meta := fmt.Sprintf("protocol=%s\nport=%d\nproto=%s\n", protocol, port, coreinstall.Proto(protocol))
		if err = safefile.Write(metaPath, []byte(meta), 0600); err != nil {
			return err
		}
	} else if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("invalid metadata while retiring core %s", name)
	}
	m, err := coreinstall.ReadMeta(name)
	if err != nil || m.Protocol != protocol || m.Port != port || m.Proto != coreinstall.Proto(protocol) {
		return fmt.Errorf("refusing to retire changed core %s", name)
	}
	if err = ops.remove(name); err != nil {
		return err
	}
	return safefile.SyncDir(coreinstall.Root)
}

func restoreReplacementOld(name string, wasActive bool, ops replacementOps) error {
	if wasActive {
		return ops.start(name)
	}
	path := filepath.Join(coreinstall.Root, name, "state.json")
	var state map[string]any
	if err := replacementRead(path, &state); err != nil {
		return err
	}
	state["disabled"] = false
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return safefile.Write(path, append(raw, '\n'), 0600)
}

func backupReplacementCore(from, to string) error {
	var dirs []string
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected core backup symlink")
		}
		dest := filepath.Join(to, rel)
		if entry.IsDir() {
			dirs = append(dirs, dest)
			return os.MkdirAll(dest, 0700)
		}
		if entry.Type()&os.ModeSocket != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 16<<20 {
			return fmt.Errorf("invalid core backup member")
		}
		return safefile.Copy(path, dest, 0600)
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err = safefile.SyncDir(dirs[i]); err != nil {
			return err
		}
	}
	return safefile.SyncDir(filepath.Dir(to))
}
