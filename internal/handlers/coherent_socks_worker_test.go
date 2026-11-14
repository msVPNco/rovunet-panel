package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCoherentSOCKSWorkerRefreshPreservesOperatorState(t *testing.T) {
	var fs syscall.Statfs_t
	if os.Getenv("ROVUNET_REVIEW25_ISOLATED") == "" || syscall.Statfs("/etc", &fs) != nil || fs.Type != 0x01021994 {
		t.Skip("private tmpfs namespace required")
	}
	unit := "/etc/systemd/system/rovunet-socks5.service"
	old, readErr := os.ReadFile(unit)
	defer func() {
		if readErr == nil {
			_ = os.WriteFile(unit, old, 0644)
		} else {
			_ = os.Remove(unit)
		}
	}()
	dir := t.TempDir()
	program := `#!/bin/sh
printf '%s\n' "$*" >> "$RV26_SOCKS_LOG"
case "$1" in
 is-active)
  printf '%s\n' "$RV26_SOCKS_STATE"
  case "$RV26_SOCKS_STATE" in active|reloading|activating) exit 0;; *) exit 3;; esac ;;
 restart) exit "$RV26_SOCKS_RESTART_EXIT" ;;
 *) exit 91 ;;
esac
`
	ok16(t, os.WriteFile(filepath.Join(dir, "systemctl"), []byte(program), 0755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	log := filepath.Join(dir, "calls")
	t.Setenv("RV26_SOCKS_LOG", log)
	for _, tc := range []struct {
		state         string
		restart, fail bool
	}{
		{"active", true, false}, {"reloading", true, false}, {"activating", true, false},
		{"inactive", false, false}, {"failed", false, false}, {"deactivating", false, false},
		{"unknown", false, false}, {"", false, true}, {"unexpected", false, true},
	} {
		t.Run(tc.state, func(t *testing.T) {
			ok16(t, os.WriteFile(unit, []byte("[Service]\nExecStart=/fixture/old-panel socks serve\n"), 0644))
			ok16(t, os.WriteFile(log, nil, 0600))
			t.Setenv("RV26_SOCKS_STATE", tc.state)
			t.Setenv("RV26_SOCKS_RESTART_EXIT", "0")
			err := refreshInstalledSOCKSWorker()
			if (err != nil) != tc.fail {
				t.Fatalf("state=%q err=%v", tc.state, err)
			}
			calls, e := os.ReadFile(log)
			ok16(t, e)
			if strings.Contains(string(calls), "restart rovunet-socks5.service") != tc.restart {
				t.Fatal(string(calls))
			}
			for _, line := range strings.Split(strings.TrimSpace(string(calls)), "\n") {
				if !strings.HasPrefix(line, "is-active ") && !strings.HasPrefix(line, "restart ") {
					t.Fatal("operator enable/start/reload state mutated", line)
				}
			}
		})
	}
	t.Setenv("RV26_SOCKS_STATE", "active")
	t.Setenv("RV26_SOCKS_RESTART_EXIT", "77")
	if refreshInstalledSOCKSWorker() == nil {
		t.Fatal("failed restart reported update success")
	}
	ok16(t, os.Remove(unit))
	ok16(t, os.WriteFile(log, nil, 0600))
	ok16(t, refreshInstalledSOCKSWorker())
	calls, e := os.ReadFile(log)
	ok16(t, e)
	if len(calls) != 0 {
		t.Fatal("missing optional service was started")
	}
}
