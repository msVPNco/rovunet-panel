package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

type fakeAccounts struct {
	exists  map[string]bool
	added   []string
	removed []string
	failOn  string
}

func fakeSSHAccounts(t *testing.T, existing ...string) *fakeAccounts {
	t.Helper()
	f := &fakeAccounts{exists: map[string]bool{}}
	for _, n := range existing {
		f.exists[n] = true
	}
	oldExists, oldAdd, oldRemove := sshAccountExists, addSSHAccount, removeSSHAccount
	t.Cleanup(func() { sshAccountExists, addSSHAccount, removeSSHAccount = oldExists, oldAdd, oldRemove })
	sshAccountExists = func(name string) bool { return f.exists[name] }
	addSSHAccount = func(name, _ string) error {
		f.added = append(f.added, name)
		f.exists[name] = true
		return nil
	}
	removeSSHAccount = func(name string) error {
		if name == f.failOn {
			return errors.New("userdel failed")
		}
		f.removed = append(f.removed, name)
		delete(f.exists, name)
		return nil
	}
	return f
}

func TestGrantSSHGivesFreeUsersAnAccountAndLimitedUsersNone(t *testing.T) {
	f := fakeSSHAccounts(t)
	if err := grantSSH(config.UserConfig{Username: "alice", Password: "p"}); err != nil {
		t.Fatal(err)
	}
	if err := grantSSH(config.UserConfig{Username: "carol", Password: "p", QuotaGB: 5}); err != nil {
		t.Fatal(err)
	}
	if len(f.added) != 1 || f.added[0] != "alice" {
		t.Fatalf("accounts created: %v, want only alice", f.added)
	}
	if len(f.removed) != 0 {
		t.Fatalf("nothing to remove for a user who never had an account: %v", f.removed)
	}
}

func TestGrantSSHTakesAwayTheAccountOfAUserWhoBecomesLimited(t *testing.T) {
	f := fakeSSHAccounts(t, "carol")
	if err := grantSSH(config.UserConfig{Username: "carol", Password: "p", QuotaGB: 5}); err != nil {
		t.Fatal(err)
	}
	if len(f.removed) != 1 || f.removed[0] != "carol" || f.exists["carol"] {
		t.Fatalf("removed=%v exists=%v, want carol's account (and sessions) removed", f.removed, f.exists)
	}
}

func TestRevokeSSHIgnoresAccountsRovunetDidNotCreate(t *testing.T) {
	// sshAccountExists is false for an account outside the rovunet-ssh group,
	// so a user record that shares a name with, say, "ubuntu" removes nothing.
	f := fakeSSHAccounts(t)
	if err := revokeSSH("ubuntu"); err != nil || len(f.removed) != 0 {
		t.Fatalf("err=%v removed=%v", err, f.removed)
	}
}

func TestSyncSSHAccessOnlyTouchesLimitedUsers(t *testing.T) {
	f := fakeSSHAccounts(t, "alice", "carol", "dave")
	cfg := &config.Config{Users: []config.UserConfig{
		{Username: "alice", Password: "p"},
		{Username: "carol", Password: "p", QuotaGB: 5},
		{Username: "dave", Password: "p", QuotaGB: 1},
	}}
	f.failOn = "dave"
	var warned []string
	syncSSHAccess(cfg, warnRecorder{&warned})
	if len(f.removed) != 1 || f.removed[0] != "carol" {
		t.Fatalf("removed %v, want only carol (alice is unlimited, dave's removal fails)", f.removed)
	}
	if len(warned) != 1 || !strings.Contains(warned[0], "dave") {
		t.Fatalf("a failed removal must be reported, got %v", warned)
	}
	if !f.exists["alice"] {
		t.Fatal("an unlimited user lost their account")
	}
}

type warnRecorder struct{ msgs *[]string }

func (w warnRecorder) Warning(m string) { *w.msgs = append(*w.msgs, m) }

func TestSOCKSCredentialsExcludeLimitedUsers(t *testing.T) {
	cfg := &config.Config{Users: []config.UserConfig{
		{Username: "alice", Password: "pa"},
		{Username: "carol", Password: "pc", QuotaGB: 5},
	}}
	got := socksCredentials(cfg)
	if len(got) != 1 || got[0].Username != "alice" {
		t.Fatalf("got %v, want only alice", got)
	}
}

func TestSOCKSStaysLockedWhenEveryUserIsLimited(t *testing.T) {
	cfg := &config.Config{Users: []config.UserConfig{{Username: "carol", Password: "pc", QuotaGB: 5}}}
	got := socksCredentials(cfg)
	if len(got) != 1 || got[0].Username != socksLockedUser || len(got[0].Password) < 32 {
		t.Fatalf("got %v, want one placeholder login so the proxy never becomes open", got)
	}
	if got[0].Password == "pc" {
		t.Fatal("the placeholder must not reuse a user's password")
	}
	if none := socksCredentials(&config.Config{}); len(none) != 0 {
		t.Fatalf("no users keeps the existing behaviour, got %v", none)
	}
}

// Every place that creates an SSH account for a panel user must go through
// grantSSH, or a traffic-limited user could get one back (a restore, a
// renewal, a password change). The wizard and the installer only ever create
// unlimited users, so they may call the system function directly.
func TestOnlyGrantSSHCreatesAccountsForPanelUsers(t *testing.T) {
	allowed := map[string]bool{"user_access.go": true, "quick_wizard.go": true, "system_install.go": true}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || allowed[f] {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "system.AddSSHUser(") {
			t.Errorf("%s calls system.AddSSHUser directly: use grantSSH so traffic-limited users get no account", f)
		}
	}
}

// The message after adding a user must not promise SSH to a limited user.
func TestUserAccessSummary(t *testing.T) {
	if got := userAccessSummary(config.UserConfig{QuotaGB: 1}); strings.Contains(got, "SSH") {
		t.Fatalf("a traffic-limited user has no SSH account, got %q", got)
	}
	for _, protocol := range []string{"SS2022", "Trojan", "VLESS Encryption"} {
		if strings.Contains(userAccessSummary(config.UserConfig{QuotaGB: 1}), protocol) {
			t.Fatalf("quota summary promised an excluded protocol: %s", protocol)
		}
	}
	if got := userAccessSummary(config.UserConfig{ExpiresAt: "2026-12-01T00:00:00Z"}); got != "SSH + SOCKS" {
		t.Fatalf("a user with only an expiry keeps SSH, got %q", got)
	}
}
