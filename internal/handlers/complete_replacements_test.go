package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

type replacementHarness struct {
	ops      replacementOps
	active   map[string]bool
	events   []string
	installs int
}

func replacementSetState(t *testing.T, name string, change func(*coreplan.State)) {
	t.Helper()
	_, st, err := replacementOptions(name)
	if err != nil {
		t.Fatal(err)
	}
	before := replacementHash(st)
	change(&st)
	if before == replacementHash(st) {
		return
	}
	raw, err := json.Marshal(st)
	if err == nil {
		err = safefile.Write(filepath.Join(coreinstall.Root, name, "state.json"), raw, 0600)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func replacementTestFixture(t *testing.T) (*config.Config, *replacementHarness) {
	t.Helper()
	isolatedSubscriptions(t)
	cfg := subTestConfig()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	var err error
	cfg, err = config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &replacementHarness{active: map[string]bool{}}
	h.ops = replacementOps{
		stop: func(name string) error {
			h.events = append(h.events, "stop:"+name)
			replacementSetState(t, name, func(st *coreplan.State) { st.Disabled = true })
			h.active[name] = false
			return nil
		},
		start: func(name string) error {
			h.events = append(h.events, "start:"+name)
			replacementSetState(t, name, func(st *coreplan.State) { st.Disabled = false })
			h.active[name] = true
			return nil
		},
		confirm: func(name string) error {
			h.events = append(h.events, "confirm:"+name)
			if !h.active[name] {
				return errors.New("fixture listener is inactive")
			}
			return nil
		},
		remove: func(name string) error {
			h.events = append(h.events, "remove:"+name)
			if _, err := coreinstall.ReadMeta(name); err != nil {
				return err
			}
			h.active[name] = false
			return os.RemoveAll(filepath.Join(coreinstall.Root, name))
		},
		status: func(name string) (string, error) {
			if h.active[name] {
				return "active", nil
			}
			return "inactive", errors.New("fixture systemctl exit status 3")
		},
		install: func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
			h.events = append(h.events, "install:"+name)
			h.installs++
			// Both old protocols used UDP. A retry must stop a restored old
			// listener before installing its TCP+UDP replacement on that port.
			for old, active := range h.active {
				m, err := coreinstall.ReadMeta(old)
				if active && err == nil && m.Port == o.Port {
					return "", errors.New("fixture port remains occupied")
				}
			}
			if err := coreplan.Validate(&o); err != nil {
				return "", err
			}
			subscriptionCoreFixture(t, name, o, users[0])
			h.active[name] = true
			return filepath.Join(coreinstall.Root, name), nil
		},
		backup: backupReplacementCore,
	}
	return cfg, h
}

func replacementAddOld(t *testing.T, cfg *config.Config, h *replacementHarness, spec completeReplacement, port int) {
	t.Helper()
	o := coreplan.Options{Protocol: spec.oldProtocol, Host: cfg.PublicAddress, Port: port, Cert: "fixture-cert", Key: "fixture-key"}
	if err := coreplan.Validate(&o); err != nil {
		t.Fatal(err)
	}
	subscriptionCoreFixture(t, spec.oldName, o, cfg.Users[0])
	h.active[spec.oldName] = true
}

func replacementSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		files[rel] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func replacementSavedRecord(t *testing.T, spec completeReplacement) replacementRecord {
	t.Helper()
	var r replacementRecord
	if err := replacementRead(filepath.Join(filepath.Dir(coreinstall.Root), "core-replacements", spec.oldName+".json"), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func replacementAssertOldRecovered(t *testing.T, h *replacementHarness, spec completeReplacement, identity string) {
	t.Helper()
	_, st, err := replacementOptions(spec.oldName)
	if err != nil || st.Disabled || !h.active[spec.oldName] || replacementIdentity(st) != identity {
		t.Fatal("failed replacement did not restore the old listener and keys")
	}
	if _, err := os.Lstat(filepath.Join(coreinstall.Root, spec.newName)); !os.IsNotExist(err) {
		t.Fatal("failed replacement directory was left behind")
	}
}

func TestCompleteReplacementKeepsPortsAccountsAndOtherCores(t *testing.T) {
	cfg, h := replacementTestFixture(t)
	oldFiles := map[string]map[string]string{}
	for i, spec := range completeReplacements {
		replacementAddOld(t, cfg, h, spec, []int{51822, 33446}[i])
		oldFiles[spec.oldName] = replacementSnapshot(t, filepath.Join(coreinstall.Root, spec.oldName))
	}
	for i, pair := range [][2]string{{"managed-awg", "amneziawg"}, {"managed-ss2022", "ss2022"}, {"custom-wireguard", "wireguard"}, {"custom-tuic", "tuic"}} {
		o := coreplan.Options{Protocol: pair[1], Host: cfg.PublicAddress, Port: 38111 + i, Cert: "fixture-cert", Key: "fixture-key", Subnet: "10.203.72.0/24", Egress: "eth0"}
		subscriptionCoreFixture(t, pair[0], o, cfg.Users[0])
		h.active[pair[0]] = true
	}
	before := replacementSnapshot(t, coreinstall.Root)
	configBefore, _ := os.ReadFile(cfg.Path())
	usersBefore := append([]config.UserConfig(nil), cfg.Users...)
	u := cfg.Users[0]
	androidURL, windowsURL := SubscriptionURL(cfg, u), WindowsSubscriptionURL(cfg, u)
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	feedBefore := savedSubscription(t, u, "android")
	blocked, err := migrateCompleteReplacementsWith(cfg, discardOutput{}, h.ops)
	if err != nil || len(blocked) != 0 || h.installs != 2 {
		t.Fatalf("replacement outcome: installed=%d blocked=%v err=%v", h.installs, blocked, err)
	}
	for i, spec := range completeReplacements {
		m, err := coreinstall.ReadMeta(spec.newName)
		if err != nil || m.Port != []int{51822, 33446}[i] || m.Protocol != spec.newProtocol || m.Proto != "tcp,udp" {
			t.Fatal("replacement did not retain the old custom port for TCP and UDP")
		}
		if _, err := os.Lstat(filepath.Join(coreinstall.Root, spec.oldName)); !os.IsNotExist(err) {
			t.Fatal("old automatic core was not retired")
		}
		r := replacementSavedRecord(t, spec)
		if r.Stage != "done" || !reflect.DeepEqual(oldFiles[spec.oldName], replacementSnapshot(t, r.Backup)) {
			t.Fatal("private backup omitted or changed original core bytes")
		}
		if err := filepath.WalkDir(r.Backup, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err == nil && info.Mode().Perm()&0077 != 0 {
				return errors.New("backup is not private")
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(h.events, "|")
		if strings.Index(joined, "confirm:"+spec.newName) > strings.Index(joined, "remove:"+spec.oldName) {
			t.Fatal("old listener was retired before replacement confirmation")
		}
	}
	for path, data := range before {
		core, _, _ := strings.Cut(path, string(filepath.Separator))
		if core == "wireguard" || core == "managed-tuic" {
			continue
		}
		now, err := os.ReadFile(filepath.Join(coreinstall.Root, path))
		if err != nil || string(now) != data {
			t.Fatal("replacement changed an unrelated core or credential")
		}
	}
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{}
	for _, item := range savedSubscription(t, u, "android").Items {
		links[item.ID] = item.Link
	}
	for _, item := range feedBefore.Items {
		if item.ID != "wireguard" && item.ID != "managed-tuic" && links[item.ID] != item.Link {
			t.Fatal("an unrelated subscription item changed")
		}
	}
	configAfter, _ := os.ReadFile(cfg.Path())
	if string(configBefore) != string(configAfter) || !reflect.DeepEqual(usersBefore, cfg.Users) || androidURL != SubscriptionURL(cfg, u) || windowsURL != WindowsSubscriptionURL(cfg, u) {
		t.Fatal("replacement rewrote an account, configuration or subscription URL")
	}
	installedFiles := replacementSnapshot(t, coreinstall.Root)
	eventCount := len(h.events)
	if _, err := migrateCompleteReplacementsWith(cfg, discardOutput{}, h.ops); err != nil || len(h.events) != eventCount || !reflect.DeepEqual(installedFiles, replacementSnapshot(t, coreinstall.Root)) {
		t.Fatal("completed replacement was not an idempotent no-op")
	}
}

func TestCompleteReplacementKeepsExplicitlyStoppedCoresOff(t *testing.T) {
	for _, flag := range []string{"disabled", "paused"} {
		t.Run(flag, func(t *testing.T) {
			cfg, h := replacementTestFixture(t)
			spec := completeReplacements[0]
			replacementAddOld(t, cfg, h, spec, 51821)
			replacementSetState(t, spec.oldName, func(st *coreplan.State) { st.Disabled, st.Paused = flag == "disabled", flag == "paused" })
			before := replacementSnapshot(t, coreinstall.Root)
			blocked, err := migrateCompleteReplacementsWith(cfg, discardOutput{}, h.ops)
			if err != nil || !blocked[spec.newProtocol] || len(h.events) != 0 || !reflect.DeepEqual(before, replacementSnapshot(t, coreinstall.Root)) {
				t.Fatal("stopped old core was enabled, replaced or omitted from the block list")
			}
		})
	}
}

func TestCompleteReplacementFailsBeforeStoppingOnCollisionOrBackupFailure(t *testing.T) {
	for _, kind := range []string{"foreign-name", "new-symlink", "wrong-old-protocol", "backup-failure", "backup-symlink", "journal-failure", "public-backup-dir"} {
		t.Run(kind, func(t *testing.T) {
			cfg, h := replacementTestFixture(t)
			spec := completeReplacements[0]
			replacementAddOld(t, cfg, h, spec, 51821)
			switch kind {
			case "foreign-name":
				subscriptionCoreFixture(t, spec.newName, coreplan.Options{Protocol: spec.newProtocol, Host: cfg.PublicAddress, Port: 51821}, cfg.Users[0])
			case "new-symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(coreinstall.Root, spec.newName)); err != nil {
					t.Fatal(err)
				}
			case "wrong-old-protocol":
				if err := os.WriteFile(filepath.Join(coreinstall.Root, spec.oldName, "meta"), []byte("protocol=tuic\nport=51821\nproto=udp\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "backup-failure":
				h.ops.backup = func(string, string) error { return errors.New("fixture backup failure") }
			case "backup-symlink":
				if err := os.Symlink("state.json", filepath.Join(coreinstall.Root, spec.oldName, "unexpected-link")); err != nil {
					t.Fatal(err)
				}
			case "journal-failure":
				h.ops.save = func(string, replacementRecord) error { return errors.New("fixture journal failure") }
			case "public-backup-dir":
				root := filepath.Join(filepath.Dir(coreinstall.Root), "core-replacements")
				if err := os.Mkdir(root, 0755); err != nil {
					t.Fatal(err)
				}
			}
			_, st, _ := replacementOptions(spec.oldName)
			if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil || len(h.events) != 0 || !h.active[spec.oldName] {
				t.Fatal("unsafe replacement did not stop before lifecycle changes")
			}
			_, after, err := replacementOptions(spec.oldName)
			if err != nil || replacementIdentity(st) != replacementIdentity(after) || after.Disabled {
				t.Fatal("preflight failure changed the old core")
			}
		})
	}
}

func TestCompleteReplacementFailureRestoresOldListener(t *testing.T) {
	for _, kind := range []string{"install-before-create", "install-after-create", "partial-install", "confirm", "stop-after-change"} {
		t.Run(kind, func(t *testing.T) {
			cfg, h := replacementTestFixture(t)
			spec := completeReplacements[0]
			replacementAddOld(t, cfg, h, spec, 51821)
			_, old, _ := replacementOptions(spec.oldName)
			install, stop := h.ops.install, h.ops.stop
			switch kind {
			case "install-before-create":
				h.ops.install = func(string, coreplan.Options, []config.UserConfig) (string, error) {
					return "", errors.New("fixture install failure")
				}
			case "install-after-create":
				h.ops.install = func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
					dir, err := install(name, o, users)
					return dir, errors.Join(err, errors.New("fixture incomplete installer cleanup"))
				}
			case "partial-install":
				h.ops.install = func(name string, _ coreplan.Options, _ []config.UserConfig) (string, error) {
					dir := filepath.Join(coreinstall.Root, name)
					if err := os.Mkdir(dir, 0700); err != nil {
						t.Fatal(err)
					}
					return dir, errors.New("fixture incomplete files")
				}
			case "confirm":
				h.ops.confirm = func(string) error { return errors.New("fixture listener confirmation failure") }
			case "stop-after-change":
				h.ops.stop = func(name string) error {
					return errors.Join(stop(name), errors.New("fixture stop failed after state write"))
				}
			}
			if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
				t.Fatal("failed replacement reported success")
			}
			replacementAssertOldRecovered(t, h, spec, replacementIdentity(old))
			if replacementSavedRecord(t, spec).Stage != "prepared" {
				t.Fatal("rollback did not record the recoverable old core")
			}
		})
	}
}

func TestCompleteReplacementResumesCrashWithoutRekeying(t *testing.T) {
	for _, point := range []string{"after-stop", "after-install", "installed-record"} {
		t.Run(point, func(t *testing.T) {
			cfg, h := replacementTestFixture(t)
			spec := completeReplacements[0]
			replacementAddOld(t, cfg, h, spec, 51821)
			install, stop := h.ops.install, h.ops.stop
			crash := errors.New("fixture process interruption")
			switch point {
			case "after-stop":
				h.ops.stop = func(name string) error {
					if err := stop(name); err != nil {
						return err
					}
					panic(crash)
				}
			case "after-install":
				h.ops.install = func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
					if _, err := install(name, o, users); err != nil {
						return "", err
					}
					panic(crash)
				}
			case "installed-record":
				h.ops.save = func(path string, r replacementRecord) error {
					if r.Stage == "installed" {
						return crash
					}
					return writeReplacementRecord(path, r)
				}
			}
			func() {
				defer func() {
					if got := recover(); got != nil && got != crash {
						panic(got)
					}
				}()
				if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
					t.Fatal("interrupted replacement reported success")
				}
			}()
			var created map[string]string
			if point != "after-stop" {
				created = replacementSnapshot(t, filepath.Join(coreinstall.Root, spec.newName))
				h.active[spec.newName] = false // the service also stopped at reboot
			}
			h.ops.install, h.ops.stop, h.ops.save = install, stop, nil
			if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err != nil {
				t.Fatal(err)
			}
			if h.installs != 1 || replacementSavedRecord(t, spec).Stage != "done" {
				t.Fatal("retry regenerated a replacement or left retirement incomplete")
			}
			if created != nil && !reflect.DeepEqual(created, replacementSnapshot(t, filepath.Join(coreinstall.Root, spec.newName))) {
				t.Fatal("crash recovery changed existing replacement keys or client files")
			}
		})
	}
}

func TestCompleteReplacementRetirementFailureResumes(t *testing.T) {
	for _, point := range []string{"before-remove", "partial-remove", "after-remove"} {
		t.Run(point, func(t *testing.T) {
			cfg, h := replacementTestFixture(t)
			spec := completeReplacements[0]
			replacementAddOld(t, cfg, h, spec, 51821)
			remove := h.ops.remove
			h.ops.remove = func(name string) error {
				if name != spec.oldName {
					return remove(name)
				}
				if point == "partial-remove" {
					if err := os.Remove(filepath.Join(coreinstall.Root, name, "meta")); err != nil {
						return err
					}
				}
				if point == "after-remove" {
					if err := remove(name); err != nil {
						return err
					}
				}
				return errors.New("fixture interrupted old retirement")
			}
			if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
				t.Fatal("failed retirement reported success")
			}
			created := replacementSnapshot(t, filepath.Join(coreinstall.Root, spec.newName))
			if replacementSavedRecord(t, spec).Stage != "retiring" || !h.active[spec.newName] {
				t.Fatal("ready replacement lost on retirement failure")
			}
			h.ops.remove = remove
			if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err != nil {
				t.Fatal(err)
			}
			if h.installs != 1 || !reflect.DeepEqual(created, replacementSnapshot(t, filepath.Join(coreinstall.Root, spec.newName))) {
				t.Fatal("retirement retry rekeyed the ready replacement")
			}
		})
	}
}

func TestCompleteReplacementCleanupFailureResumes(t *testing.T) {
	cfg, h := replacementTestFixture(t)
	spec := completeReplacements[0]
	replacementAddOld(t, cfg, h, spec, 51821)
	remove, confirm := h.ops.remove, h.ops.confirm
	h.ops.confirm = func(string) error { return errors.New("fixture failed confirmation") }
	h.ops.remove = func(name string) error {
		if name != spec.newName {
			return remove(name)
		}
		if err := os.Remove(filepath.Join(coreinstall.Root, name, "meta")); err != nil {
			return err
		}
		return errors.New("fixture partial replacement cleanup")
	}
	if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
		t.Fatal("failed cleanup reported success")
	}
	if replacementSavedRecord(t, spec).Stage != "rolling-back" || h.active[spec.oldName] {
		t.Fatal("cleanup intent was lost or old port restarted before cleanup")
	}
	h.ops.remove, h.ops.confirm = remove, confirm
	if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err != nil {
		t.Fatal(err)
	}
	if replacementSavedRecord(t, spec).Stage != "done" || h.installs != 2 {
		t.Fatal("cleanup retry did not finish the per-core replacement")
	}
}

func TestCompleteReplacementRestopsOldAfterRollbackJournalFailure(t *testing.T) {
	cfg, h := replacementTestFixture(t)
	spec := completeReplacements[0]
	replacementAddOld(t, cfg, h, spec, 51821)
	install := h.ops.install
	failed := false
	h.ops.install = func(string, coreplan.Options, []config.UserConfig) (string, error) {
		failed = true
		return "", errors.New("fixture install failure")
	}
	h.ops.save = func(path string, r replacementRecord) error {
		if failed && (r.Stage == "rolling-back" || r.Stage == "prepared") {
			return errors.New("fixture rollback journal failure")
		}
		return writeReplacementRecord(path, r)
	}
	if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
		t.Fatal("failed journal reported success")
	}
	if !h.active[spec.oldName] || replacementSavedRecord(t, spec).Stage != "stopped" {
		t.Fatal("fixture did not reach restored-old/stale-journal boundary")
	}
	h.ops.install, h.ops.save = install, nil
	if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteReplacementRejectsChangedRecordedTarget(t *testing.T) {
	for _, kind := range []string{"key-change", "directory-replaced"} {
		t.Run(kind, func(t *testing.T) {
			cfg, h := replacementTestFixture(t)
			spec := completeReplacements[0]
			replacementAddOld(t, cfg, h, spec, 51821)
			h.ops.save = func(path string, r replacementRecord) error {
				if r.Stage == "installed" {
					return errors.New("fixture journal interruption")
				}
				return writeReplacementRecord(path, r)
			}
			if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
				t.Fatal("fixture did not interrupt")
			}
			if kind == "key-change" {
				replacementSetState(t, spec.newName, func(st *coreplan.State) { st.Seed = strings.Repeat("cd", 32) })
			} else {
				path := filepath.Join(coreinstall.Root, spec.newName)
				if err := os.Rename(path, path+"-saved"); err != nil {
					t.Fatal(err)
				}
				r := replacementSavedRecord(t, spec)
				subscriptionCoreFixture(t, spec.newName, replacementTargetOptions(r), cfg.Users[0])
			}
			before := replacementSnapshot(t, filepath.Join(coreinstall.Root, spec.newName))
			h.ops.save = nil
			if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
				t.Fatal("changed target was silently adopted")
			}
			if !reflect.DeepEqual(before, replacementSnapshot(t, filepath.Join(coreinstall.Root, spec.newName))) || h.installs != 1 {
				t.Fatal("unrecognized target was changed or reinstalled")
			}
		})
	}
}

func TestCompleteReplacementInactiveOldRollbackDoesNotStartIt(t *testing.T) {
	cfg, h := replacementTestFixture(t)
	spec := completeReplacements[0]
	replacementAddOld(t, cfg, h, spec, 51821)
	h.active[spec.oldName] = false
	h.ops.install = func(string, coreplan.Options, []config.UserConfig) (string, error) {
		return "", fmt.Errorf("fixture install failure")
	}
	if _, err := replaceCompleteCore(cfg, discardOutput{}, spec, h.ops); err == nil {
		t.Fatal("failed install reported success")
	}
	_, state, err := replacementOptions(spec.oldName)
	if err != nil || state.Disabled || h.active[spec.oldName] {
		t.Fatal("rollback enabled an originally inactive core or left a new disabled flag")
	}
}
