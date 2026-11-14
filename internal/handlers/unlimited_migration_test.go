package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/usage"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func unlimitedFixture(t *testing.T) (*config.Config, string, string) {
	t.Helper()
	c := fixture16(t)
	c.Users = c.Users[:1]
	c.Users[0].UsageBase = 30
	ok16(t, c.Save())
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	old := usage.Dir
	usage.Dir = t.TempDir()
	t.Cleanup(func() { usage.Dir = old })
	p := filepath.Join(usage.Dir, "vless.json")
	b, _ := json.Marshal(map[string]int64{c.Users[0].Username: 200})
	ok16(t, os.WriteFile(p, b, 0600))
	return c, filepath.Join(t.TempDir(), "migration"), p
}
func TestUnlimitedMigrationIdentityAndUsage(t *testing.T) {
	c, d, p := unlimitedFixture(t)
	old := c.Users[0]
	feed := func() [][]byte {
		var rows [][]byte
		for _, f := range []string{"json", "windows.json"} {
			b, e := buildFeedForClient(c, c.Users[0], map[string]string{"json": "", "windows.json": "windows"}[f])
			ok16(t, e)
			rows = append(rows, b)
		}
		return rows
	}
	before := feed()
	ok16(t, PlanUnlimitedMigration(c, d))
	ok16(t, ApplyUnlimitedMigration(d))
	ok16(t, ApplyUnlimitedMigration(d))
	next, e := config.LoadFrom(c.Path())
	ok16(t, e)
	c = next
	u := c.Users[0]
	if u.Username != old.Username || u.Password != old.Password || u.SubToken != old.SubToken || u.AccountID != old.AccountID || u.PeriodID != old.PeriodID || u.UsageBase != old.UsageBase || u.QuotaGB != 0 || u.ExpiresAt != old.ExpiresAt {
		t.Fatal("identity/policy changed")
	}
	after := feed()
	for i := range before {
		if string(before[i]) != string(after[i]) {
			t.Fatal("feed bytes changed")
		}
	}
	b, _ := json.Marshal(map[string]int64{u.Username: 250})
	ok16(t, os.WriteFile(p, b, 0600))
	ok16(t, ObserveUnlimitedAccounts(c))
	ok16(t, ObserveUnlimitedAccounts(c))
	a, lim, e := boundAmount(c.ServerID, u)
	ok16(t, e)
	if a.Confirmed != 220 || lim != 0 || !rotationEligible(c, u) {
		t.Fatal(a, lim)
	}
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	ok16(t, InitializeV2Authority(c))
	snap, e := ResolveV2FromConfig(c.Path(), nil, "")(context.Background(), u.SubToken, "status.json")
	ok16(t, e)
	var v StatusV2
	ok16(t, json.Unmarshal(snap.Body, &v))
	if v.LimitKind != "unlimited" || v.Limit != nil || v.Status != "active" || v.Confirmed == nil || *v.Confirmed != 220 || v.Coverage != "partial" {
		t.Fatal(v)
	}
	b, _ = json.Marshal(map[string]int64{u.Username: 249})
	ok16(t, os.WriteFile(p, b, 0600))
	if ObserveUnlimitedAccounts(c) == nil {
		t.Fatal("regression accepted")
	}
	a, _, e = boundAmount(c.ServerID, u)
	ok16(t, e)
	if a.Confirmed != 220 {
		t.Fatal(a)
	}
	ok16(t, os.Rename(u.SQLiteMeter.Directory, u.SQLiteMeter.Directory+".retained"))
	snap, e = ResolveV2FromConfig(c.Path(), nil, "")(context.Background(), u.SubToken, "status.json")
	ok16(t, e)
	ok16(t, json.Unmarshal(snap.Body, &v))
	if v.Status != "hold" || v.Confirmed != nil {
		t.Fatal("missing DB looked healthy", v)
	}
}
func TestUnlimitedMigrationInterruptions(t *testing.T) {
	for _, phase := range []string{"intent", "imported", "bootstrapped", "selected", "receipt"} {
		t.Run(phase, func(t *testing.T) {
			c, d, _ := unlimitedFixture(t)
			ok16(t, PlanUnlimitedMigration(c, d))
			unlimitedMigrationBarrier = func(p string) error {
				if p == phase {
					return errors.New("interrupted")
				}
				return nil
			}
			e := ApplyUnlimitedMigration(d)
			unlimitedMigrationBarrier = nil
			if e == nil {
				t.Fatal("injection missing")
			}
			ok16(t, ApplyUnlimitedMigration(d))
			ok16(t, ApplyUnlimitedMigration(d))
			fresh, e := config.LoadFrom(c.Path())
			ok16(t, e)
			a, _, e := boundAmount(fresh.ServerID, fresh.Users[0])
			ok16(t, e)
			if a.Confirmed != 170 {
				t.Fatal(a)
			}
		})
	}
}
func TestUnlimitedMigrationRefusals(t *testing.T) {
	for _, kind := range []string{"changed-usage", "tampered-policy", "tampered-intent", "lost-complete-db", "missing-evidence", "finite-account"} {
		t.Run(kind, func(t *testing.T) {
			c, d, p := unlimitedFixture(t)
			if kind == "finite-account" {
				c.Users[0].QuotaGB = 1
				ok16(t, c.Save())
				if PlanUnlimitedMigration(c, d) == nil {
					t.Fatal("finite accepted")
				}
				return
			}
			if kind == "missing-evidence" {
				ok16(t, os.Remove(p))
				if PlanUnlimitedMigration(c, d) == nil {
					t.Fatal("invented zero")
				}
				return
			}
			ok16(t, PlanUnlimitedMigration(c, d))
			switch kind {
			case "changed-usage":
				ok16(t, os.WriteFile(p, []byte("{\"alice\":201}"), 0600))
			case "tampered-policy":
				plan, e := loadUnlimitedPlan(d)
				ok16(t, e)
				var top map[string]any
				ok16(t, json.Unmarshal(plan.After, &top))
				top["public_address"] = "evil.invalid"
				plan.After, _ = json.Marshal(top)
				b, _ := json.Marshal(plan)
				ok16(t, os.WriteFile(filepath.Join(d, "plan.json"), b, 0600))
			case "tampered-intent":
				ok16(t, os.WriteFile(filepath.Join(d, "apply.pending"), []byte("other"), 0600))
			case "lost-complete-db":
				ok16(t, ApplyUnlimitedMigration(d))
				ok16(t, os.Rename(filepath.Join(d, "db"), filepath.Join(d, "db.retained")))
			}
			if ApplyUnlimitedMigration(d) == nil {
				t.Fatal("unsafe migration accepted")
			}
		})
	}
}
func TestUnlimitedMigrationKillHelper(t *testing.T) {
	if os.Getenv("RV23_KILL_PHASE") == "" {
		t.Skip("child helper")
	}
	unlimitedMigrationBarrier = func(p string) error {
		if p == os.Getenv("RV23_KILL_PHASE") {
			proc, _ := os.FindProcess(os.Getpid())
			proc.Kill()
		}
		return nil
	}
	usage.Dir = os.Getenv("RV23_USAGE")
	if e := ApplyUnlimitedMigration(os.Getenv("RV23_PLAN")); e != nil {
		t.Fatal(e)
	}
}
func TestUnlimitedMigrationRealKill(t *testing.T) {
	for _, phase := range []string{"intent", "imported", "bootstrapped", "selected", "receipt"} {
		t.Run(phase, func(t *testing.T) {
			c, d, _ := unlimitedFixture(t)
			ok16(t, PlanUnlimitedMigration(c, d))
			child := exec.Command(os.Args[0], "-test.run=^TestUnlimitedMigrationKillHelper$")
			child.Env = append(os.Environ(), "RV23_KILL_PHASE="+phase, "RV23_PLAN="+d, "RV23_USAGE="+usage.Dir)
			err := child.Run()
			var death *exec.ExitError
			if !errors.As(err, &death) || death.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatalf("expected SIGKILL, got %v", err)
			}
			ok16(t, ApplyUnlimitedMigration(d))
			ok16(t, ApplyUnlimitedMigration(d))
			fresh, e := config.LoadFrom(c.Path())
			ok16(t, e)
			s, e := quota.Open(context.Background(), fresh.Users[0].SQLiteMeter.Directory, fresh.ServerID, fresh.Users[0].SQLiteMeter.Store)
			ok16(t, e)
			defer s.Close()
			m, e := s.Metadata(context.Background(), quota.Key{Account: fresh.Users[0].AccountID, Period: fresh.Users[0].PeriodID})
			ok16(t, e)
			if m.Confirmed != 170 || m.Pending != 0 {
				t.Fatal(m)
			}
		})
	}
}

func TestUnlimitedMigrationKeepsAllTwentyFourProfiles(t *testing.T) {
	c, _ := twentyProfileSubscriptionFixture(t)
	addManagedSubscriptionFixture(t, c)
	oldUsage := usage.Dir
	usage.Dir = t.TempDir()
	defer func() { usage.Dir = oldUsage }()
	b, _ := json.Marshal(map[string]int64{c.Users[0].Username: 9007199254740993})
	ok16(t, os.WriteFile(filepath.Join(usage.Dir, "vless.json"), b, 0600))
	before := map[string]string{}
	for _, client := range []string{"", "windows"} {
		body, e := buildFeedForClient(c, c.Users[0], client)
		ok16(t, e)
		var f subFeed
		ok16(t, json.Unmarshal(body, &f))
		if len(f.Items) != 24 {
			t.Fatal(len(f.Items))
		}
		before[client] = string(body)
	}
	oldURL := SubscriptionURL(c, c.Users[0])
	oldWindows := WindowsSubscriptionURL(c, c.Users[0])
	dir := filepath.Join(t.TempDir(), "plan")
	ok16(t, PlanUnlimitedMigration(c, dir))
	ok16(t, ApplyUnlimitedMigration(dir))
	fresh, e := config.LoadFrom(c.Path())
	ok16(t, e)
	if SubscriptionURL(fresh, fresh.Users[0]) != oldURL || WindowsSubscriptionURL(fresh, fresh.Users[0]) != oldWindows {
		t.Fatal("URL changed")
	}
	for _, client := range []string{"", "windows"} {
		body, e := buildFeedForClient(fresh, fresh.Users[0], client)
		ok16(t, e)
		if string(body) != before[client] {
			t.Fatal("24-profile bytes changed")
		}
	}
	a, _, e := boundAmount(fresh.ServerID, fresh.Users[0])
	ok16(t, e)
	if a.Confirmed != 9007199254740993 {
		t.Fatal("int64 rounded", a)
	}
}
