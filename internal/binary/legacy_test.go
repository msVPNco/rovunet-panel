package binary

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStandaloneRuntimeRejectsUnverifiedOfflineBytesWithoutReplacingOld(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "caddy-naive")
	old := []byte("previous complete runtime")
	if err := os.WriteFile(dest, old, 0755); err != nil {
		t.Fatal(err)
	}
	offline := t.TempDir()
	if err := os.WriteFile(filepath.Join(offline, "caddy-naive"), []byte("untrusted"), 0755); err != nil {
		t.Fatal(err)
	}
	previous := OfflineDir
	OfflineDir = offline
	t.Cleanup(func() { OfflineDir = previous })
	if err := InstallReviewedTransportAt("caddy-naive", dest); err == nil {
		t.Fatal("accepted unsigned arbitrary offline runtime")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(old) {
		t.Fatalf("old runtime changed: %q %v", got, err)
	}
	if _, err := os.Stat(dest + ".bak"); !os.IsNotExist(err) {
		t.Fatal("failed verification wrote a backup")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dest, link); err != nil {
		t.Fatal(err)
	}
	if err := InstallReviewedTransportAt("caddy-naive", link); err == nil {
		t.Fatal("accepted symlink destination")
	}
}
