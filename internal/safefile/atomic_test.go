package safefile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFailedWriteKeepsPrivateBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup")
	if err := os.WriteFile(path, []byte("healthy"), 0644); err != nil {
		t.Fatal(err)
	}
	errDisk := errors.New("no space left on device")
	err := Replace(path, 0600, func(w io.Writer) error {
		_, _ = io.WriteString(w, "partial")
		return errDisk
	})
	if !errors.Is(err, errDisk) {
		t.Fatalf("got %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "healthy" {
		t.Fatal("existing file was damaged")
	}
	if err := Write(path, []byte("complete"), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("private mode not applied")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("left partial staging files")
	}
}

func TestAtomicReplacementKeepsServiceOwnership(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("ownership change fixture requires isolated Linux root")
	}
	p := filepath.Join(t.TempDir(), "service-config")
	os.WriteFile(p, []byte("old"), 0600)
	if err := os.Chown(p, 65534, 65534); err != nil {
		t.Fatal(err)
	}
	if err := Write(p, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(p)
	st := info.Sys().(*syscall.Stat_t)
	if st.Uid != 65534 || st.Gid != 65534 {
		t.Fatal("atomic update broke service file ownership")
	}
}

func TestNeverFollowsDestinationSymlink(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "other")
	path := filepath.Join(dir, "backup")
	os.WriteFile(secret, []byte("unrelated"), 0600)
	if err := os.Symlink(secret, path); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("replacement"), 0600); err == nil {
		t.Fatal("followed symlink")
	}
	got, _ := os.ReadFile(secret)
	if string(got) != "unrelated" {
		t.Fatal("changed unrelated target")
	}
}
