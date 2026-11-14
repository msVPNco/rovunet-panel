package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestCoherentUpdateCannotSkipTransportAuthentication(t *testing.T) {
	var fs syscall.Statfs_t
	if os.Getenv("ROVUNET_REVIEW25_ISOLATED") == "" || syscall.Statfs("/usr/local/bin", &fs) != nil || fs.Type != 0x01021994 {
		t.Skip("private tmpfs namespace required")
	}
	c, _, _ := bound19(t)
	before, e := json.Marshal(c.Users)
	ok16(t, e)
	path := filepath.Join(config.DefaultBinDir, "dnstt-server")
	old, readErr := os.ReadFile(path)
	defer func() {
		if readErr == nil {
			_ = os.WriteFile(path, old, 0755)
		} else {
			_ = os.Remove(path)
		}
	}()
	retained := []byte("unreviewed existing transport fixture")
	ok16(t, os.WriteFile(path, retained, 0755))
	previous := binary.OfflineDir
	binary.OfflineDir = t.TempDir()
	t.Cleanup(func() { binary.OfflineDir = previous })
	e = HandlePostUpdate(&actions.Context{Config: c, Output: discardOutput{}})
	if e == nil {
		t.Fatal("SQLite-backed update skipped missing reviewed transport bytes")
	}
	got, e := os.ReadFile(path)
	ok16(t, e)
	if string(got) != string(retained) {
		t.Fatal("failed staging replaced existing executable")
	}
	after, e := json.Marshal(c.Users)
	ok16(t, e)
	if string(before) != string(after) {
		t.Fatal("failed transport update changed account identity or accounting binding")
	}
}
