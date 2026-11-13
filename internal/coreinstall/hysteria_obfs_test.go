package coreinstall

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

func sameObfsFiles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			return false
		}
	}
	return true
}

func TestAutomaticHysteriaMigrationRetainsExplicitOffAndDoesNotRestartTwice(t *testing.T) {
	oldRoot, oldCheck, oldStatus, oldRestart, oldReady := Root, hy2CheckConfig, hy2Status, hy2Restart, hy2Ready
	t.Cleanup(func() {
		Root, hy2CheckConfig, hy2Status, hy2Restart, hy2Ready = oldRoot, oldCheck, oldStatus, oldRestart, oldReady
	})
	Root = filepath.Join(t.TempDir(), "core")
	users := []config.UserConfig{free}
	newUserCore(t, "existing-hy2", hy2, users)
	if err := saveApplied(filepath.Join(Root, "existing-hy2")); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	hy2CheckConfig = func(string) error { return nil }
	hy2Status = func(string) (string, error) { return "active", nil }
	hy2Restart = func(string) error { restarts++; return nil }
	hy2Ready = func(string) error { return nil }
	for i := 0; i < 2; i++ {
		if err := ConfigureDefaultHysteriaObfuscation(users); err != nil {
			t.Fatal(err)
		}
	}
	o, _, _, _ := loadUserCore("existing-hy2")
	if !o.HysteriaSalamander || !o.HysteriaObfsConfigured || restarts != 1 {
		t.Fatal("automatic migration was not idempotent")
	}
	if _, err := ChangeHysteriaObfuscation("existing-hy2", false, users); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureDefaultHysteriaObfuscation(users); err != nil {
		t.Fatal(err)
	}
	o, _, _, _ = loadUserCore("existing-hy2")
	if o.HysteriaSalamander || !o.HysteriaObfsConfigured || restarts != 2 {
		t.Fatal("explicit operator choice was lost")
	}
}

func TestHysteriaModeValidationRollbackAndIdempotence(t *testing.T) {
	oldRoot, oldCheck, oldStatus, oldRestart, oldReady := Root, hy2CheckConfig, hy2Status, hy2Restart, hy2Ready
	t.Cleanup(func() {
		Root, hy2CheckConfig, hy2Status, hy2Restart, hy2Ready = oldRoot, oldCheck, oldStatus, oldRestart, oldReady
	})
	Root = filepath.Join(t.TempDir(), "core")
	users := []config.UserConfig{free}
	newUserCore(t, "hy2", hy2, users)
	newUserCore(t, "unrelated", reality, users)
	dir := filepath.Join(Root, "hy2")
	if err := saveApplied(dir); err != nil {
		t.Fatal(err)
	}
	if err := saveApplied(filepath.Join(Root, "unrelated")); err != nil {
		t.Fatal(err)
	}
	before, _ := obfsSnapshot(dir)
	unrelated, _ := obfsSnapshot(filepath.Join(Root, "unrelated"))
	restarts := 0
	hy2Status = func(string) (string, error) { return "active", nil }
	hy2Ready = func(string) error { return nil }
	hy2Restart = func(string) error { restarts++; return nil }
	hy2CheckConfig = func(string) error { return errors.New("fixture rejected") }
	if backup, err := ChangeHysteriaObfuscation("hy2", true, users); err == nil || backup != "" {
		t.Fatal("invalid config was applied")
	}
	after, _ := obfsSnapshot(dir)
	if !sameObfsFiles(before, after) || restarts != 0 {
		t.Fatal("validation changed existing state")
	}
	hy2CheckConfig = func(string) error { return nil }
	backup, err := ChangeHysteriaObfuscation("hy2", true, users)
	if err != nil || backup == "" || restarts != 1 {
		t.Fatal(backup, err, restarts)
	}
	recovery, err := obfsSnapshot(backup)
	if err != nil || !sameObfsFiles(before, recovery) {
		t.Fatal("recovery does not match prior files", err)
	}
	masked, _ := obfsSnapshot(dir)
	o, _, _, _ := loadUserCore("hy2")
	if !o.HysteriaSalamander {
		t.Fatal("enabled option not persisted")
	}
	if backup, err := ChangeHysteriaObfuscation("hy2", true, users); err != nil || backup != "" || restarts != 1 {
		t.Fatal("repeat was not a no-op", err)
	}
	calls := 0
	hy2Restart = func(string) error {
		calls++
		if calls == 1 {
			return errors.New("fixture startup failure")
		}
		return nil
	}
	if backup, err := ChangeHysteriaObfuscation("hy2", false, users); err == nil || backup == "" || calls != 2 {
		t.Fatal("failed startup did not roll back", err, calls)
	}
	after, _ = obfsSnapshot(dir)
	if !sameObfsFiles(masked, after) {
		t.Fatal("failed mode change altered credentials or applied files")
	}
	hy2Restart = func(string) error { return nil }
	if _, err := ChangeHysteriaObfuscation("hy2", false, users); err != nil {
		t.Fatal(err)
	}
	after, _ = obfsSnapshot(dir)
	var explicitChoice coreplan.Options
	if err := json.Unmarshal(before[optionsFile], &explicitChoice); err != nil {
		t.Fatal(err)
	}
	explicitChoice.HysteriaObfsConfigured = true
	data, _ := json.MarshalIndent(explicitChoice, "", "  ")
	before[optionsFile] = append(data, '\n')
	if !sameObfsFiles(before, after) {
		t.Fatal("disabling did not restore exact plain configuration")
	}
	otherAfter, _ := obfsSnapshot(filepath.Join(Root, "unrelated"))
	if !sameObfsFiles(unrelated, otherAfter) {
		t.Fatal("unrelated core changed")
	}
	// Applying the option offline must retain an administrative stop.
	o, stopped, _, _ := loadUserCore("hy2")
	stopped.Disabled = true
	if err := saveState(dir, &stopped); err != nil {
		t.Fatal(err)
	}
	hy2Status = func(string) (string, error) { return "inactive", errors.New("inactive") }
	hy2Restart = func(string) error { t.Fatal("stopped core was restarted"); return nil }
	if _, err := ChangeHysteriaObfuscation("hy2", true, users); err != nil {
		t.Fatal(err)
	}
	o, stopped, _, _ = loadUserCore("hy2")
	if !o.HysteriaSalamander || !stopped.Disabled {
		t.Fatal("administrative stop not preserved")
	}
	if _, err := ChangeHysteriaObfuscation("hy2", false, users); err != nil {
		t.Fatal(err)
	}
	if _, err := ChangeHysteriaObfuscation("unrelated", true, users); err == nil {
		t.Fatal("wrong protocol accepted")
	}
	if _, err := ChangeHysteriaObfuscation("../hy2", true, users); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "server.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ChangeHysteriaObfuscation("hy2", true, users); err == nil {
		t.Fatal("unapplied state accepted")
	}
}
