package network

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnedPortsPreserveSharedAndPreexistingRules(t *testing.T) {
	oldFile, oldAllow, oldRemove, oldExists := ownerFile, ownedAllow, ownedRemove, ownedRuleExists
	t.Cleanup(func() { ownerFile, ownedAllow, ownedRemove, ownedRuleExists = oldFile, oldAllow, oldRemove, oldExists })
	ownerFile = filepath.Join(t.TempDir(), "owners.json")
	removed := 0
	ownedAllow = func(int, string) error { return nil }
	ownedRemove = func(int, string) error { removed++; return nil }
	ownedRuleExists = func(int, string) bool { return false }
	if err := AllowOwnedPort("a", 45678, "tcp,udp"); err != nil {
		t.Fatal(err)
	}
	if err := AllowOwnedPort("b", 45678, "tcp"); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseOwnedPort("a", 45678, "tcp,udp"); err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatal("a shared TCP rule was removed or an exclusive UDP rule remained")
	}
	if err := ReleaseOwnedPort("b", 45678, "tcp"); err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatal("last owner did not release its rule")
	}
	ownedRuleExists = func(int, string) bool { return true }
	if err := AllowOwnedPort("c", 45679, "tcp"); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseOwnedPort("c", 45679, "tcp"); err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatal("a preexisting rule was removed")
	}
}

func TestOwnedPortsRecordPartialAllowAndRetryFailedRemoval(t *testing.T) {
	oldFile, oldAllow, oldRemove, oldExists := ownerFile, ownedAllow, ownedRemove, ownedRuleExists
	t.Cleanup(func() { ownerFile, ownedAllow, ownedRemove, ownedRuleExists = oldFile, oldAllow, oldRemove, oldExists })
	ownerFile = filepath.Join(t.TempDir(), "owners.json")
	ownedRuleExists = func(int, string) bool { return false }
	ownedAllow = func(_ int, proto string) error {
		if proto == "udp" {
			return errors.New("UDP failure")
		}
		return nil
	}
	if AllowOwnedPort("a", 45678, "tcp,udp") == nil {
		t.Fatal("partial failure was hidden")
	}
	b, err := os.ReadFile(ownerFile)
	if err != nil || !strings.Contains(string(b), "45678/tcp") {
		t.Fatal("successful TCP operation was not recorded")
	}
	ownedRemove = func(int, string) error { return errors.New("remove failure") }
	if ReleaseOwnedPort("a", 45678, "tcp,udp") == nil {
		t.Fatal("remove failure was hidden")
	}
	removed := 0
	ownedRemove = func(int, string) error { removed++; return nil }
	if err := ReleaseOwnedPort("a", 45678, "tcp,udp"); err != nil || removed != 1 {
		t.Fatal("failed removal could not be retried", err)
	}
}
