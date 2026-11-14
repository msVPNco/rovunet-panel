package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallIncludesTimerAndOrphanedEnablement(t *testing.T) {
	dir := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	script := "#!/bin/sh\necho \"$*\" >> \"$UNIT_CLEANUP_LOG\"\ncase \"$1\" in list-units) echo 'rovunet-user-expiry.timer loaded active waiting Expiry';; esac\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "systemctl"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("UNIT_CLEANUP_LOG", log)
	for _, name := range []string{"rovunet-core-test.service", "rovunet-user-expiry.timer", "slipgate-old.service", "unrelated.service"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	wants := filepath.Join(dir, "timers.target.wants")
	if err := os.Mkdir(wants, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../rovunet-user-expiry.timer", filepath.Join(wants, "rovunet-user-expiry.timer")); err != nil {
		t.Fatal(err)
	}
	if err := removePanelUnits(dir); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"rovunet-core-test.service", "rovunet-user-expiry.timer", "slipgate-old.service", "timers.target.wants/rovunet-user-expiry.timer"} {
		if _, err := os.Lstat(filepath.Join(dir, path)); !os.IsNotExist(err) {
			t.Errorf("left %s", path)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "unrelated.service")); err != nil {
		t.Fatal("removed unrelated unit")
	}
	calls, _ := os.ReadFile(log)
	a, b := strings.Index(string(calls), "stop rovunet-user-expiry.timer"), strings.Index(string(calls), "stop rovunet-core-test.service")
	if a < 0 || b < 0 || a > b {
		t.Fatal("timer was not stopped before workers")
	}
}
