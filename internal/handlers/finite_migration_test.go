package handlers

import (
	"encoding/json"
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestFiniteMigrationWithServiceOwnedConfigDirectory(t *testing.T) {
	var fs syscall.Statfs_t
	if os.Getenv("ROVUNET_REVIEW25_ISOLATED") == "" || syscall.Statfs("/etc", &fs) != nil || fs.Type != 0x01021994 {
		t.Skip("private tmpfs namespace required")
	}
	c, _ := finiteFixture25(t)
	old, readErr := os.ReadFile(config.DefaultConfigFile)
	ok16(t, os.MkdirAll(config.DefaultConfigDir, 0750))
	info, e := os.Stat(config.DefaultConfigDir)
	ok16(t, e)
	st := info.Sys().(*syscall.Stat_t)
	defer func() {
		_ = os.Chown(config.DefaultConfigDir, int(st.Uid), int(st.Gid))
		if readErr == nil {
			_ = os.WriteFile(config.DefaultConfigFile, old, 0600)
		} else {
			_ = os.Remove(config.DefaultConfigFile)
		}
	}()
	ok16(t, c.SaveTo(config.DefaultConfigFile))
	ok16(t, os.Chown(config.DefaultConfigDir, 999, 988))
	ok16(t, BindFiniteAccounting(c))
	if !strings.HasPrefix(c.Users[0].SQLiteMeter.Directory, "/var/lib/rovunet-quota/finite-") {
		t.Fatal("finite authority under service-owned configuration")
	}
	a, _, e := boundAmount(c.ServerID, c.Users[0])
	ok16(t, e)
	if a.Confirmed != 170 {
		t.Fatal("recorded opening lost", a)
	}
	ok16(t, BindFiniteAccounting(c))
	u := c.Users[0]
	u.QuotaGB = 0
	u.SQLiteMeter = nil
	b, e := installPeriod(config.DefaultConfigFile, c.ServerID, u, 200)
	ok16(t, e)
	if !strings.HasPrefix(b.Directory, "/var/lib/rovunet-quota/accounts-") {
		t.Fatal("unlimited authority under service-owned configuration")
	}
}

func TestFiniteVMKillHelper(t *testing.T) {
	phase := os.Getenv("ROVUNET_FINITE_VM_KILL_PHASE")
	if phase == "" {
		t.Skip("isolated VM process-kill acceptance")
	}
	if _, e := os.Stat("/etc/rovunet-lab-authorized"); e != nil {
		t.Fatal("owned isolated VM marker required")
	}
	c, e := config.Load()
	ok16(t, e)
	// Exercise SelfPath's installed-binary fallback, not a test executable in
	// generated service units. This private disposable copy is uploaded per run.
	p, e := os.Executable()
	ok16(t, e)
	if filepath.Dir(p) != "/root/act25" || !strings.HasPrefix(filepath.Base(p), "handlers-") || !strings.HasSuffix(p, ".test") {
		t.Fatal("disposable VM helper path required")
	}
	ok16(t, os.Remove(p))
	finiteMigrationBarrier = func(p string) error {
		if p == phase {
			proc, e := os.FindProcess(os.Getpid())
			if e != nil {
				return e
			}
			return proc.Kill()
		}
		return nil
	}
	defer func() { finiteMigrationBarrier = nil }()
	ok16(t, MigrateFiniteAccounting(c, &actions.StdOutput{}))
	t.Fatal("kill phase was not reached")
}

func finiteFixture25(t *testing.T) (*config.Config, string) {
	t.Helper()
	c, _, p := unlimitedFixture(t)
	secret := filepath.Join(t.TempDir(), "secret")
	ok16(t, os.WriteFile(secret, []byte(strings.Repeat("17", 32)), 0600))
	c.Tunnels = append(c.Tunnels, config.TunnelConfig{Tag: "vless", Transport: config.TransportVLESS, Enabled: true, Domain: "n.example.com", VLESS: &config.VLESSConfig{Port: 2053, Path: "/ws", Front: "naive", SecretFile: secret, TLS: true}})
	c.Users[0].QuotaGB = 25
	ok16(t, c.Save())
	return c, p
}
func TestFiniteBindingPreservesIdentityUsageAndReplay(t *testing.T) {
	c, _ := finiteFixture25(t)
	before := c.Users[0]
	url := SubscriptionURL(c, before)
	ok16(t, BindFiniteAccounting(c))
	ok16(t, BindFiniteAccounting(c))
	u := c.Users[0]
	if u.SQLiteMeter == nil {
		t.Fatal("not bound")
	}
	a, limit, e := boundAmount(c.ServerID, u)
	ok16(t, e)
	if a.Confirmed != 170 || limit != 25*(1<<30) {
		t.Fatal(a, limit)
	}
	u.SQLiteMeter = nil
	if !reflect.DeepEqual(before, u) || url != SubscriptionURL(c, u) {
		t.Fatal("account policy, credentials or URL changed")
	}
	// A lost bound store must never be recreated as zero on replay.
	dir := c.Users[0].SQLiteMeter.Directory
	ok16(t, os.Rename(dir, dir+".retained"))
	if BindFiniteAccounting(c) == nil {
		t.Fatal("lost ledger replaced")
	}
}
func TestFiniteBindingDoesNotInventMissingConsumption(t *testing.T) {
	c, p := finiteFixture25(t)
	ok16(t, os.Remove(p))
	if BindFiniteAccounting(c) == nil {
		t.Fatal("missing usage accepted as zero")
	}
	if c.Users[0].SQLiteMeter != nil {
		t.Fatal("partially selected account")
	}
}
func TestFiniteBindingPreservesPublishedFloor(t *testing.T) {
	c, _ := finiteFixture25(t)
	u := c.Users[0]
	n := int64(9007199254740993)
	st := accountStatus{AccountID: u.AccountID, PeriodID: u.PeriodID, UsedBytes: &n}
	b, e := json.Marshal(st)
	ok16(t, e)
	ok16(t, os.WriteFile(filepath.Join(publicDir(), u.SubToken+".status.json"), b, 0644))
	ok16(t, BindFiniteAccounting(c))
	a, _, e := boundAmount(c.ServerID, c.Users[0])
	ok16(t, e)
	if a.Confirmed != n {
		t.Fatal("published int64 usage floor regressed", a)
	}
}

func TestBoundFiniteAutomaticSetupAddsOnlyEnforceableBridges(t *testing.T) {
	c, _ := finiteFixture25(t)
	oldURL := SubscriptionURL(c, c.Users[0])
	ok16(t, BindFiniteAccounting(c))
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, c, &installed, &backups)
	ok16(t, ensureCompleteSetupWith(c, discardOutput{}, ops))
	ok16(t, ensureCompleteSetupWith(c, discardOutput{}, ops))
	if len(installed) != 5 || backups != 1 {
		t.Fatal("finite bridge setup/replay", installed, backups)
	}
	for _, client := range []string{"android", "windows"} {
		feed := savedSubscription(t, c.Users[0], client)
		if len(feed.Items) != 6 {
			t.Fatal("expected native WS plus five quota bridge profiles", client, len(feed.Items))
		}
	}
	if oldURL != SubscriptionURL(c, c.Users[0]) {
		t.Fatal("subscription URL changed")
	}
	a, _, e := boundAmount(c.ServerID, c.Users[0])
	ok16(t, e)
	if a.Confirmed != 170 {
		t.Fatal("setup changed recorded usage", a)
	}
}
