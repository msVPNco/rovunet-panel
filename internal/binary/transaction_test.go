package binary

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestInterruptedUpdateKeepsRecoveryAndResumesNewPanel(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "rovunet")
	candidate := filepath.Join(d, "candidate")
	os.WriteFile(target, []byte("healthy previous panel"), 0755)
	os.WriteFile(candidate, []byte("authenticated new panel"), 0755)
	tx, err := BeginUpdate(target, candidate, filepath.Join(d, "private-snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.SavePhase("post-update-pending"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(target + ".bak")
	if string(b) != "healthy previous panel" {
		t.Fatal("recovery binary lost before post-update completion")
	}
	got, resume, err := ResumeUpdate(target)
	if err != nil || !resume || got.Phase != "post-update-pending" {
		t.Fatalf("resume=%v: %v", resume, err)
	}
	os.WriteFile(candidate, []byte("another candidate"), 0755)
	if _, err := BeginUpdate(target, candidate, "other snapshot"); err == nil {
		t.Fatal("overwrote pending recovery")
	}
	if err := tx.SavePhase("complete"); err != nil {
		t.Fatal(err)
	}
	if _, resume, err := ResumeUpdate(target); err != nil || resume {
		t.Fatal("completed update reran migration")
	}
	if _, err := os.Stat(target + ".bak"); err != nil {
		t.Fatal("successful update discarded operator recovery binary")
	}
}

func TestCrossFilesystemFailureNeverRemovesWorkingPanel(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "rovunet")
	os.WriteFile(target, []byte("healthy"), 0755)
	f, err := os.CreateTemp("/dev/shm", "rovunet-staged-test-*")
	if err != nil {
		t.Skip("no isolated shared-memory filesystem")
	}
	defer os.Remove(f.Name())
	f.WriteString("verified candidate")
	f.Close()
	var a, b syscall.Stat_t
	syscall.Stat(target, &a)
	syscall.Stat(f.Name(), &b)
	if a.Dev == b.Dev {
		t.Skip("EXDEV scenario requires different filesystems")
	}
	if _, err := BeginUpdate(target, f.Name(), "private snapshot"); err == nil || !strings.Contains(err.Error(), "atomic replacement failed") {
		t.Fatalf("expected retained old file: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "healthy" {
		t.Fatal("working panel was removed or truncated")
	}
	if _, resume, err := ResumeUpdate(target); err != nil || resume {
		t.Fatal("pre-install crash could not recover to healthy previous panel")
	}
	tx, _ := ReadUpdateTransaction(target)
	if tx.Phase != "aborted-before-install" {
		t.Fatal("failed install reported success")
	}
}
