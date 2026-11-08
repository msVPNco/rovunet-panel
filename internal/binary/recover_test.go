package binary

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

// The case seen on a real server: the old binary moved to .bak, the download
// left an empty .tmp, and the update stopped. The old binary comes back.
func TestRecoverInterruptedRestoresTheOldBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "vaydns-server")
	write(t, bin+".bak", "old binary")
	write(t, bin+".tmp", "")
	restored, err := RecoverInterrupted(bin)
	if err != nil || !restored {
		t.Fatalf("restored=%v err=%v", restored, err)
	}
	got, _ := os.ReadFile(bin)
	if string(got) != "old binary" {
		t.Fatalf("binary is %q", got)
	}
	if exists(bin+".tmp") || exists(bin+".bak") {
		t.Fatal("the partial download must go, and the backup was moved back")
	}
	if st, _ := os.Stat(bin); st.Mode().Perm()&0100 == 0 {
		t.Fatal("the restored binary must stay executable")
	}
}

func TestRecoverInterruptedLeavesABinaryInPlace(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "dnstt-server")
	write(t, bin, "new binary")
	write(t, bin+".bak", "old binary")
	write(t, bin+".tmp", "half")
	restored, err := RecoverInterrupted(bin)
	if err != nil || restored {
		t.Fatalf("restored=%v err=%v", restored, err)
	}
	if got, _ := os.ReadFile(bin); string(got) != "new binary" {
		t.Fatalf("a binary in place must not change, got %q", got)
	}
	if !exists(bin + ".bak") {
		t.Fatal("a backup is never removed here")
	}
	if exists(bin + ".tmp") {
		t.Fatal("a partial download is removed")
	}
}

func TestRecoverInterruptedNeedsAUsableBackup(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "caddy-naive")
	if restored, err := RecoverInterrupted(bin); err != nil || restored {
		t.Fatalf("not installed: restored=%v err=%v", restored, err)
	}
	write(t, bin+".bak", "") // an empty backup is no backup
	if restored, err := RecoverInterrupted(bin); err != nil || restored || exists(bin) {
		t.Fatalf("an empty backup must not be restored: %v %v", restored, err)
	}
}
