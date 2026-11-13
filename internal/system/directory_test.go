package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirRejectsSymlinkWithoutChangingTarget(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "managed")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(path, "root"); err == nil {
		t.Fatal("symbolic directory accepted")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link changed")
	}
	info, err = os.Stat(target)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("target changed")
	}
}
