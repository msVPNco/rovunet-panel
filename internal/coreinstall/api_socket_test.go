package coreinstall

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareAPISocketAfterPowerCut(t *testing.T) {
	old := Root
	Root = t.TempDir()
	t.Cleanup(func() { Root = old })
	dir := filepath.Join(Root, "fixture")
	os.Mkdir(dir, 0700)
	path := filepath.Join(dir, "api.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := prepareAPISocket("fixture", path); err == nil {
		t.Fatal("live listener accepted for replacement")
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal("live socket removed")
	}
	listener.Close()
	if err := prepareAPISocket("fixture", path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("stale socket survived")
	}
	replacement, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal("recovery could not bind the original endpoint", err)
	}
	replacement.Close()
}

func TestPrepareAPISocketPreservesNonOwnedObjects(t *testing.T) {
	old := Root
	Root = t.TempDir()
	t.Cleanup(func() { Root = old })
	dir := filepath.Join(Root, "fixture")
	os.Mkdir(dir, 0700)
	path := filepath.Join(dir, "api.sock")
	if err := os.WriteFile(path, []byte("must survive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareAPISocket("fixture", path); err == nil {
		t.Fatal("regular file removed")
	}
	if b, _ := os.ReadFile(path); string(b) != "must survive" {
		t.Fatal("regular file changed")
	}
	os.Remove(path)
	target := filepath.Join(t.TempDir(), "private")
	os.WriteFile(target, []byte("outside"), 0600)
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := prepareAPISocket("fixture", path); err == nil {
		t.Fatal("symlink accepted")
	}
	if b, _ := os.ReadFile(target); string(b) != "outside" {
		t.Fatal("symlink target changed")
	}
	if err := prepareAPISocket("fixture", target); err == nil {
		t.Fatal("external endpoint accepted")
	}
}
