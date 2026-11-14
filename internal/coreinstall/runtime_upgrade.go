package coreinstall

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

//go:embed reviewed-release-runtime-pins.json
var reviewedReleaseRuntimePins []byte

type runtimeUpgradeFile struct {
	Asset  string `json:"asset"`
	Target string `json:"target"`
	Staged string `json:"staged"`
	Backup string `json:"backup"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type runtimeUpgradeJournal struct {
	Schema      int                  `json:"schema"`
	Phase       string               `json:"phase"`
	Files       []runtimeUpgradeFile `json:"files"`
	ActiveUnits []string             `json:"active_units"`
}

var stageReleaseRuntime = func(asset, dir string) (string, error) {
	return binary.Download(binary.RuntimeDownloadBase()+"/"+asset, dir)
}
var runtimeUpgradeRestart = func(unit string) error {
	if err := collectCore(strings.TrimPrefix(unit, "rovunet-core-")); err != nil && !errors.Is(err, usage.ErrUnsampledTraffic) {
		return fmt.Errorf("preserve runtime usage before restart: %w", err)
	}
	if err := service.Restart(unit); err != nil {
		return err
	}
	state, err := service.Status(unit)
	if err != nil || state != "active" {
		return fmt.Errorf("runtime service did not become active: %s", unit)
	}
	return nil
}
var runtimeUpgradeValidate = validateStagedRuntimeChanges
var runtimeUpgradeRecord = func(files []runtimeUpgradeFile) error {
	awg := false
	for _, f := range files {
		switch f.Target {
		case XrayPath, dedicatedXrayPath():
			if err := verifyXrayFile(f.Target); err != nil {
				return err
			}
		case AWGGoPath, AWGToolPath, awgManifestPath():
			awg = true
		}
	}
	if awg {
		return recordAWG()
	}
	return nil
}

func runtimeAssetTargets() map[string]string {
	suffix := "-linux-" + runtime.GOARCH
	return map[string]string{
		XrayPath: "xray" + suffix, dedicatedXrayPath(): "xray" + suffix,
		AWGGoPath: "amneziawg-go" + suffix, AWGToolPath: "awg" + suffix,
		awgManifestPath():            "awg-provenance" + suffix + ".json",
		supplementalPath("sing-box"): "sing-box" + suffix, supplementalPath("mita"): "mita" + suffix,
	}
}
func coreExecutable(c Meta) string {
	if c.Protocol == "amneziawg" {
		return AWGGoPath
	}
	if kind := coreplan.SupplementalRuntime(c.Protocol); kind != "" {
		return supplementalPath(kind)
	}
	return XrayForCore(c.Name)
}
func validateStagedRuntimeChanges(files []runtimeUpgradeFile) ([]string, error) {
	replacements := map[string]string{}
	for _, f := range files {
		replacements[f.Target] = f.Staged
		switch {
		case f.Target == XrayPath || f.Target == dedicatedXrayPath():
			if err := verifyXrayFile(f.Staged); err != nil {
				return nil, err
			}
			defer os.Remove(f.Staged + ".provenance.json")
		case f.Target == AWGGoPath:
			if err := verifyGoSource(f.Staged, AWGGoCommit); err != nil {
				return nil, err
			}
		case f.Target == supplementalPath("sing-box"):
			if err := verifySupplemental("sing-box", f.Staged); err != nil {
				return nil, err
			}
		case f.Target == supplementalPath("mita"):
			if err := verifySupplemental("mita", f.Staged); err != nil {
				return nil, err
			}
		}
	}
	cores, err := List()
	if err != nil {
		return nil, err
	}
	active := []string{}
	for _, c := range cores {
		if err := ValidateName(c.Name); err != nil {
			return nil, err
		}
		executable, changed := replacements[coreExecutable(c)]
		if c.Protocol == "amneziawg" && !changed {
			// An AWG tool/manifest change also requires rechecking its configuration.
			_, changed = replacements[AWGToolPath]
			if !changed {
				_, changed = replacements[awgManifestPath()]
			}
		}
		if !changed {
			continue
		}
		configPath := filepath.Join(Root, c.Name, "server.json")
		if c.Protocol == "amneziawg" {
			if _, err := readAWG(configPath); err != nil {
				return nil, err
			}
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			var cmd *exec.Cmd
			switch coreplan.SupplementalRuntime(c.Protocol) {
			case "sing-box":
				cmd = exec.CommandContext(ctx, executable, "check", "-c", configPath)
			case "mita":
				cmd = exec.CommandContext(ctx, "unshare", "--mount", "--net", "--pid", "--fork", "--kill-child", "sh", "-c", mitaValidationScript, "mita-validation", executable, configPath)
			default:
				cmd = exec.CommandContext(ctx, executable, "run", "-test", "-c", configPath)
			}
			_, err := cmd.CombinedOutput()
			cancel()
			if err != nil {
				return nil, fmt.Errorf("reviewed runtime rejected core %s configuration; existing runtimes preserved", c.Name)
			}
		}
		state, err := service.Status(ServiceName(c.Name))
		switch state {
		case "active":
			if err != nil {
				return nil, err
			}
			active = append(active, ServiceName(c.Name))
		case "inactive", "failed", "unknown": // Preserve an administrator's stopped unit.
		default:
			return nil, fmt.Errorf("cannot snapshot runtime service state for %s", c.Name)
		}
	}
	return active, nil
}

// RefreshInstalledEngines is an explicit update operation, never normal boot
// admission. Stage/authenticate/validate everything before the first rename.
// Its durable journal resumes across a crash between any rename and restart;
// per-content backups survive completion and stopped units remain stopped.
func RefreshInstalledEngines() error {
	targets := runtimeAssetTargets()
	var pins []struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(reviewedReleaseRuntimePins, &pins); err != nil {
		return err
	}
	expected := map[string]string{}
	for _, p := range pins {
		expected[p.Name] = p.SHA256
	}
	if _, err := os.Stat(Root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(Root, ".runtime-upgrade.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	path := filepath.Join(Root, ".runtime-upgrade.json")
	save := func(j *runtimeUpgradeJournal) error {
		b, err := json.Marshal(j)
		if err != nil {
			return err
		}
		return safefile.Write(path, append(b, '\n'), 0600)
	}
	var j runtimeUpgradeJournal
	data, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(data, &j) != nil || j.Schema != 1 {
			return fmt.Errorf("invalid runtime update recovery record")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if j.Phase == "" || j.Phase == "complete" {
		j = runtimeUpgradeJournal{Schema: 1, Phase: "staged"}
		keep := false
		defer func() {
			if !keep {
				for _, f := range j.Files {
					_ = os.Remove(f.Staged)
				}
			}
		}()
		for target, asset := range targets {
			st, err := os.Lstat(target)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if !st.Mode().IsRegular() {
				return fmt.Errorf("runtime target must be a regular file")
			}
			after := expected[asset]
			if len(after) != 64 {
				return fmt.Errorf("runtime asset has no embedded reviewed digest: %s", asset)
			}
			before, err := binary.FileSHA256(target)
			if err != nil {
				return err
			}
			if before == after {
				continue
			}
			temp, err := stageReleaseRuntime(asset, filepath.Dir(target))
			if err != nil {
				return fmt.Errorf("stage %s; existing runtimes retained: %w", asset, err)
			}
			got, err := binary.FileSHA256(temp)
			if err != nil || got != after {
				_ = os.Remove(temp)
				return fmt.Errorf("staged runtime differs from embedded review pin: %s", asset)
			}
			j.Files = append(j.Files, runtimeUpgradeFile{Asset: asset, Target: target, Staged: temp, Backup: target + ".before-" + before, Before: before, After: after})
		}
		if len(j.Files) == 0 {
			return nil
		}
		j.ActiveUnits, err = runtimeUpgradeValidate(j.Files)
		if err != nil {
			return err
		}
		if err = save(&j); err != nil {
			return err
		}
		keep = true
	}
	if j.Phase != "staged" && j.Phase != "installed" {
		return fmt.Errorf("unknown runtime update recovery phase")
	}
	for _, unit := range j.ActiveUnits {
		if !strings.HasPrefix(unit, "rovunet-core-") || ValidateName(strings.TrimPrefix(unit, "rovunet-core-")) != nil {
			return fmt.Errorf("invalid runtime recovery unit")
		}
	}
	for _, f := range j.Files {
		if targets[f.Target] != f.Asset || expected[f.Asset] != f.After || len(f.Before) != 64 || f.Backup != f.Target+".before-"+f.Before || filepath.Dir(f.Staged) != filepath.Dir(f.Target) {
			return fmt.Errorf("invalid runtime recovery paths or review pins")
		}
		current, err := binary.FileSHA256(f.Target)
		if err != nil {
			return err
		}
		if current == f.After {
			if hash, err := binary.FileSHA256(f.Backup); err != nil || hash != f.Before {
				return fmt.Errorf("installed runtime recovery backup missing or changed")
			}
			continue
		}
		if current != f.Before {
			return fmt.Errorf("runtime changed outside pending update; preserve recovery files")
		}
		if hash, err := binary.FileSHA256(f.Backup); os.IsNotExist(err) {
			if err = safefile.Copy(f.Target, f.Backup, 0755); err != nil {
				return err
			}
		} else if err != nil || hash != f.Before {
			return fmt.Errorf("runtime recovery backup mismatch")
		}
		hash, err := binary.FileSHA256(f.Staged)
		if err != nil || hash != f.After {
			return fmt.Errorf("staged runtime unavailable; preserve recovery record and backups")
		}
		if err = os.Rename(f.Staged, f.Target); err != nil {
			return err
		}
		if err = safefile.SyncDir(filepath.Dir(f.Target)); err != nil {
			return err
		}
	}
	j.Phase = "installed"
	if err = save(&j); err != nil {
		return err
	}
	if err = runtimeUpgradeRecord(j.Files); err != nil {
		return fmt.Errorf("installed runtime provenance pending; retry update to resume: %w", err)
	}
	for _, unit := range j.ActiveUnits {
		if err = runtimeUpgradeRestart(unit); err != nil {
			return fmt.Errorf("runtime installed, restart pending; retry update to resume: %w", err)
		}
	}
	j.Phase = "complete"
	return save(&j)
}
