package coreinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRuntimeUpgradeStagingResumeAndRecovery(t *testing.T) {
	if os.Getenv("ROVUNET_REVIEW25_ISOLATED") == "" {
		t.Skip("requires disposable runtime namespace")
	}
	for _, mode := range []string{"stage-failure", "validation-failure", "success", "stopped", "restart-retry", "rename-crash", "unrelated-change", "changed-backup"} {
		t.Run(mode, func(t *testing.T) {
			oldRoot, oldXray, oldGo, oldTool := Root, XrayPath, AWGGoPath, AWGToolPath
			oldPins, oldStage, oldValidate, oldRestart, oldRecord := reviewedReleaseRuntimePins, stageReleaseRuntime, runtimeUpgradeValidate, runtimeUpgradeRestart, runtimeUpgradeRecord
			t.Cleanup(func() {
				Root, XrayPath, AWGGoPath, AWGToolPath = oldRoot, oldXray, oldGo, oldTool
				reviewedReleaseRuntimePins, stageReleaseRuntime, runtimeUpgradeValidate, runtimeUpgradeRestart = oldPins, oldStage, oldValidate, oldRestart
				runtimeUpgradeRecord = oldRecord
			})
			Root = t.TempDir()
			bin := t.TempDir()
			XrayPath = filepath.Join(bin, "xray")
			AWGGoPath = filepath.Join(bin, "awg-go")
			AWGToolPath = filepath.Join(bin, "awg-tool")
			before, after := []byte("old complete runtime"), []byte("new reviewed runtime")
			digest := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
			if err := os.WriteFile(XrayPath, before, 0755); err != nil {
				t.Fatal(err)
			}
			asset := "xray-linux-" + runtime.GOARCH
			reviewedReleaseRuntimePins, _ = json.Marshal([]map[string]string{{"name": asset, "sha256": digest(after)}})
			staged := ""
			runtimeUpgradeRecord = func([]runtimeUpgradeFile) error { return nil }
			downloads, restarts := 0, 0
			stageReleaseRuntime = func(got, dir string) (string, error) {
				downloads++
				if got != asset {
					t.Fatalf("unplanned asset %s", got)
				}
				if mode == "stage-failure" {
					return "", errors.New("authenticated download failed")
				}
				f, err := os.CreateTemp(dir, ".runtime-candidate-")
				if err != nil {
					return "", err
				}
				staged = f.Name()
				if _, err = f.Write(after); err != nil {
					f.Close()
					return "", err
				}
				if err = f.Close(); err != nil {
					return "", err
				}
				return staged, os.Chmod(staged, 0755)
			}
			runtimeUpgradeValidate = func(files []runtimeUpgradeFile) ([]string, error) {
				if len(files) != 1 {
					t.Fatal("unexpected staging group")
				}
				if mode == "validation-failure" {
					return nil, errors.New("existing private configuration rejected")
				}
				if mode == "stopped" {
					return nil, nil
				}
				return []string{"rovunet-core-on"}, nil
			}
			runtimeUpgradeRestart = func(unit string) error {
				if unit != "rovunet-core-on" {
					t.Fatalf("restarted unplanned/stopped unit: %s", unit)
				}
				restarts++
				if (mode == "restart-retry" || mode == "rename-crash" || mode == "unrelated-change" || mode == "changed-backup") && restarts == 1 {
					return errors.New("fixture interrupted after installation")
				}
				return nil
			}
			err := RefreshInstalledEngines()
			switch mode {
			case "stage-failure", "validation-failure":
				if err == nil {
					t.Fatal("failed staging/validation reported success")
				}
				data, _ := os.ReadFile(XrayPath)
				if string(data) != string(before) || restarts != 0 {
					t.Fatal("failed preparation changed the old runtime/service")
				}
				if _, err := os.Stat(filepath.Join(Root, ".runtime-upgrade.json")); !os.IsNotExist(err) {
					t.Fatal("preparation failure left a committed journal")
				}
				if staged != "" {
					if _, err := os.Stat(staged); !os.IsNotExist(err) {
						t.Fatal("uncommitted candidate not cleaned")
					}
				}
				return
			case "success", "stopped":
				if err != nil {
					t.Fatal(err)
				}
			default:
				if err == nil {
					t.Fatal("restart interruption was not reported")
				}
			}
			backup := XrayPath + ".before-" + digest(before)
			data, _ := os.ReadFile(backup)
			if string(data) != string(before) {
				t.Fatal("complete old runtime recovery copy lost")
			}
			if mode == "rename-crash" {
				p := filepath.Join(Root, ".runtime-upgrade.json")
				data, _ := os.ReadFile(p)
				var j runtimeUpgradeJournal
				if err := json.Unmarshal(data, &j); err != nil {
					t.Fatal(err)
				}
				j.Phase = "staged"
				data, _ = json.Marshal(j)
				if err := os.WriteFile(p, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unrelated-change" {
				if err := os.WriteFile(XrayPath, []byte("external modification"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "changed-backup" {
				if err := os.WriteFile(backup, []byte("damaged backup"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			err = RefreshInstalledEngines()
			if mode == "unrelated-change" || mode == "changed-backup" {
				if err == nil {
					t.Fatal("overwrote unrelated executable or ignored damaged recovery")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, _ = os.ReadFile(XrayPath)
			if string(data) != string(after) {
				t.Fatal("reviewed runtime not installed")
			}
			if downloads != 1 {
				t.Fatal("retry downloaded another candidate instead of resuming")
			}
			if mode == "stopped" && restarts != 0 {
				t.Fatal("administrator stopped service restarted")
			}
			if mode == "success" && restarts != 1 {
				t.Fatal("idempotent retry restarted healthy service")
			}
			p := filepath.Join(Root, ".runtime-upgrade.json")
			data, _ = os.ReadFile(p)
			var j runtimeUpgradeJournal
			_ = json.Unmarshal(data, &j)
			if j.Phase != "complete" {
				t.Fatal("runtime transaction not completed")
			}
		})
	}
}
