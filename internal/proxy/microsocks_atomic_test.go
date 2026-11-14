package proxy

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSocksCredentialAtomicReplacementDoesNotFollowSymlinks(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "credentials")
	secret := filepath.Join(d, "private")
	if err := os.WriteFile(secret, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, target+".tmp"); err != nil {
		t.Fatal(err)
	}
	old := syscall.Umask(0077)
	defer syscall.Umask(old)
	if err := writeSocksCredentials(target, []byte("fixture:credential\n"), -1, -1); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal(info, err)
	}
	raw, _ := os.ReadFile(secret)
	if string(raw) != "sentinel" {
		t.Fatal("fixed staging symlink followed")
	}
	if err = os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(secret, target); err != nil {
		t.Fatal(err)
	}
	if writeSocksCredentials(target, []byte("must not write"), -1, -1) == nil {
		t.Fatal("destination symlink accepted")
	}
	raw, _ = os.ReadFile(secret)
	if string(raw) != "sentinel" {
		t.Fatal("destination target overwritten")
	}
	leftovers, _ := filepath.Glob(filepath.Join(d, ".rovunet-socks-*"))
	if len(leftovers) != 0 {
		t.Fatal("staging leaked", leftovers)
	}
}
