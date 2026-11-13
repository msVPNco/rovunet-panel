package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSSHAccountOperationsRefuseAdministratorAndUnmanagedAccounts(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "mutation")
	// All commands are fixtures. No OS accounts or sessions are changed.
	id := "#!/bin/sh\ncase \"$*\" in\n'-u root') echo 0;;\n'-u admin') echo 1000;;\n'-u alice') echo 1234;;\n'-Gn root'|'-Gn admin') echo sudo;;\n'-Gn alice') echo rovunet-ssh;;\nroot|admin|alice) exit 0;;\n*) exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "id"), []byte(id), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pkill", "userdel", "chpasswd", "groupadd", "useradd"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n: > \"$SSH_MUTATION_MARKER\"\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("SSH_MUTATION_MARKER", marker)
	for _, name := range []string{"root", "admin", "-unsafe"} {
		if err := AddSSHUser(name, "synthetic-password"); err == nil {
			t.Errorf("changed password of %s", name)
		}
		if err := RemoveSSHUser(name); err == nil {
			t.Errorf("removed %s", name)
		}
		if err := TerminateSSHUserSessions(name); err == nil {
			t.Errorf("killed sessions of %s", name)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("unowned account command ran")
	}
	if err := TerminateSSHUserSessions("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("owned session command did not run")
	}
}
