package coreinstall

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestBootPreparationCannotRestoreExpiredOrDisabledCredentials(t *testing.T) {
	old := Root
	Root = t.TempDir()
	t.Cleanup(func() { Root = old })
	user := config.UserConfig{Username: "alice", Password: "fixture"}
	newUserCore(t, "hy2", hy2, []config.UserConfig{user})
	user.ExpiresAt = "2000-01-01T00:00:00Z"
	if Prepare("hy2", []config.UserConfig{user}) == nil {
		t.Fatal("an expired-only core was allowed to boot")
	}
	if _, err := os.Stat(filepath.Join(Root, "hy2", "clients", "alice.txt")); !os.IsNotExist(err) {
		t.Fatal("expired credential returned in boot preparation")
	}
	_, st, _, _ := loadUserCore("hy2")
	st.Disabled = true
	if err := saveState(filepath.Join(Root, "hy2"), &st); err != nil {
		t.Fatal(err)
	}
	user.ExpiresAt = ""
	if Prepare("hy2", []config.UserConfig{user}) == nil {
		t.Fatal("administratively stopped core was allowed to boot")
	}
}

func TestRepeatedManagedInstallPreservesKeysAndAddresses(t *testing.T) {
	old := Root
	Root = t.TempDir()
	t.Cleanup(func() { Root = old })
	fakeSeams(t)
	users := []config.UserConfig{{Username: "alice", Password: "fixture"}}
	newUserCore(t, "hy2", hy2, users)
	_, before, _, _ := loadUserCore("hy2")
	if _, err := InstallForUsers("hy2", hy2, users); err != nil {
		t.Fatal(err)
	}
	_, after, _, _ := loadUserCore("hy2")
	if before.Seed != after.Seed || before.ServerPrivate != after.ServerPrivate {
		t.Fatal("repeat install rotated existing credentials")
	}
}
