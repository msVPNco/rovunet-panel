package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

var statusNow = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

func statusTestInput(totals map[string]int64, collected, complete bool) statusInput {
	c := usage.Collection{Tried: statusNow.Add(-2 * time.Minute), Complete: complete}
	if complete {
		c.At = c.Tried
	}
	return statusInput{serverID: "srv", now: statusNow, totals: totals, totalsOK: true, collection: c, collected: collected}
}

func TestStatusOfUnlimitedUser(t *testing.T) {
	st := buildStatus(statusTestInput(map[string]int64{"a": 999}, true, true), config.UserConfig{Username: "a", AccountID: "acc", PeriodID: "per"}, "")
	if st.Status != statusActive || st.ExpiresAt != nil || st.LimitBytes != nil || st.UsedBytes != nil || st.SampledAt != nil {
		t.Fatalf("an unlimited user has no expiry, limit or count: %+v", st)
	}
	if st.Metering != meteringNone || st.ServerID != "srv" || st.AccountID != "acc" || st.PeriodID != "per" || st.Version != 1 {
		t.Fatalf("got %+v", st)
	}
}

func TestStatusOfLimitedUser(t *testing.T) {
	u := config.UserConfig{Username: "a", QuotaGB: 2, UsageBase: 100, ExpiresAt: "2026-12-01T00:00:00Z"}
	st := buildStatus(statusTestInput(map[string]int64{"a": 100 + 5<<20}, true, true), u, "")
	if st.Status != statusActive || *st.LimitBytes != 2<<30 || *st.UsedBytes != 5<<20 {
		t.Fatalf("used counts from the renewal base, limit in bytes: %+v", st)
	}
	if *st.ExpiresAt != "2026-12-01T00:00:00Z" || st.Metering != meteringComplete || *st.SampledAt != "2026-11-08T09:58:00Z" {
		t.Fatalf("got %+v", st)
	}
	if st := buildStatus(statusTestInput(nil, true, false), u, ""); st.Metering != meteringPartial || st.SampledAt != nil {
		t.Fatalf("a core that did not answer makes the count partial, with no complete sample: %+v", st)
	}
	in := statusTestInput(nil, true, false)
	in.collection.At = statusNow.Add(-time.Hour) // the last complete read
	if st := buildStatus(in, u, ""); st.Metering != meteringPartial || *st.SampledAt != "2026-11-08T09:00:00Z" {
		t.Fatalf("a failed read must not make the old count look fresh: %+v", st)
	}
	in = statusTestInput(nil, true, true)
	in.totalsOK = false
	if st := buildStatus(in, u, ""); st.Metering != meteringPartial {
		t.Fatalf("an unreadable totals file makes the count partial, got %s", st.Metering)
	}
	if st := buildStatus(statusTestInput(nil, false, false), u, ""); st.Metering != meteringUnavailable || st.SampledAt != nil {
		t.Fatalf("never read: unavailable, no sample time, got %+v", st)
	}
}

// What the app shows must match what the next limits check will do, and why
// a stopped user was stopped.
func TestStatusSaysWhyAccessEnds(t *testing.T) {
	in := statusTestInput(map[string]int64{"a": 3 << 30}, true, true)
	cases := []struct {
		u      config.UserConfig
		reason string
		want   string
	}{
		{config.UserConfig{Username: "a", ExpiresAt: "2026-11-01T00:00:00Z"}, "", statusExpired},
		{config.UserConfig{Username: "a", QuotaGB: 2}, "", statusLimitReached},
		{config.UserConfig{Username: "a", QuotaGB: 5}, "", statusActive},
		{config.UserConfig{Username: "a", QuotaGB: 5}, "expired", statusExpired},
		{config.UserConfig{Username: "a", QuotaGB: 5}, "traffic limit", statusLimitReached},
	}
	for i, c := range cases {
		if got := buildStatus(in, c.u, c.reason).Status; got != c.want {
			t.Errorf("case %d: got %s, want %s", i, got, c.want)
		}
	}
}

func TestStatusRevisionChangesOnlyWithContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.status.json")
	u := config.UserConfig{Username: "a", QuotaGB: 2}
	rev := func(totals map[string]int64, at time.Time) int64 {
		in := statusTestInput(totals, true, true)
		in.now, in.collection.At, in.collection.Tried = at, at, at
		body, err := statusBody(buildStatus(in, u, ""), path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0644); err != nil {
			t.Fatal(err)
		}
		var st accountStatus
		if err := json.Unmarshal(body, &st); err != nil {
			t.Fatal(err)
		}
		return st.Revision
	}
	if r := rev(map[string]int64{"a": 10}, statusNow); r != 1 {
		t.Fatalf("first file: revision %d", r)
	}
	if r := rev(map[string]int64{"a": 10}, statusNow.Add(5*time.Minute)); r != 1 {
		t.Fatalf("only the times changed: revision %d", r)
	}
	if r := rev(map[string]int64{"a": 20}, statusNow.Add(10*time.Minute)); r != 2 {
		t.Fatalf("the count changed: revision %d", r)
	}
}

func TestStatusKeepsKnownLowerBoundOnReadFailureButAllowsRenewal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account.status.json")
	u := config.UserConfig{Username: "a", AccountID: "account", PeriodID: "first", QuotaGB: 1}
	previous := buildStatus(statusTestInput(map[string]int64{"a": 2 << 30}, true, true), u, "")
	old, err := statusBody(previous, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	body, err := statusBody(buildStatus(statusTestInput(nil, false, false), u, ""), path)
	if err != nil {
		t.Fatal(err)
	}
	var st accountStatus
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatal(err)
	}
	if st.UsedBytes == nil || *st.UsedBytes != 2<<30 || st.Metering != meteringPartial || st.Status != statusLimitReached {
		t.Fatal("failed read erased a known count or made the account healthy")
	}
	u.PeriodID = "renewed"
	body, err = statusBody(buildStatus(statusTestInput(nil, false, false), u, ""), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatal(err)
	}
	if st.UsedBytes == nil || *st.UsedBytes != 0 || st.Metering != meteringUnavailable || st.Status != statusActive {
		t.Fatal("renewal did not start a new period, or broke the released Android numeric count contract")
	}
}

// The status file carries exactly the documented keys, and a stopped user
// keeps it while the subscription goes.
func TestWriteSubscriptionsStatusFiles(t *testing.T) {
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subs")
	usage.Dir = t.TempDir()
	defer func() { usage.Dir = "/var/lib/rovunet/usage" }()
	cfg := subTestConfig()
	stopped := cfg.Users[1]
	cfg.Users = cfg.Users[:1]
	cfg.Archived = []config.ArchivedUser{{UserConfig: stopped, Reason: "traffic limit", At: "2026-11-08T09:00:00Z"}}
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(transport.SubscriptionDir, "sub")
	raw, err := os.ReadFile(filepath.Join(dir, cfg.Users[0].SubToken+statusSuffix))
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range top {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := "account_id account_status expires_at generated_at limit_bytes metering_status period_id revision sampled_at server_id used_bytes version"
	if strings.Join(keys, " ") != want {
		t.Fatalf("keys %v", keys)
	}
	if strings.Contains(string(raw), "pw-alice") || strings.Contains(string(raw), "rovunet://") {
		t.Fatal("the status file grants nothing: no password, no config")
	}
	if _, err := os.Stat(filepath.Join(dir, stopped.SubToken+".json")); err == nil {
		t.Fatal("a stopped user must not keep a subscription")
	}
	raw, err = os.ReadFile(filepath.Join(dir, stopped.SubToken+statusSuffix))
	if err != nil {
		t.Fatal("a stopped user keeps the status file:", err)
	}
	var st accountStatus
	if err := json.Unmarshal(raw, &st); err != nil || st.Status != statusLimitReached || st.AccountID != stopped.AccountID {
		t.Fatalf("got %+v (%v)", st, err)
	}

	// Removed for good: nothing is left.
	cfg.Archived = nil
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, stopped.SubToken+statusSuffix)); err == nil {
		t.Fatal("a removed user's status file must go")
	}
}

// A config from before the identifiers existed gets them once, saved, and
// they never change after that.
func TestIdentifiersAreMadeOnceAndKept(t *testing.T) {
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subs")
	usage.Dir = t.TempDir()
	defer func() { usage.Dir = "/var/lib/rovunet/usage" }()
	old := subTestConfig()
	old.ServerID = ""
	old.Users[0].AccountID, old.Users[0].PeriodID = "", ""
	old.Archived = []config.ArchivedUser{{UserConfig: config.UserConfig{Username: "carol", SubToken: strings.Repeat("c4", 16)}, Reason: "expired"}}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := old.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	saved, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	u, a := saved.Users[0], saved.Archived[0]
	if len(saved.ServerID) != 32 || len(u.AccountID) != 32 || len(u.PeriodID) != 32 || len(a.AccountID) != 32 || len(a.PeriodID) != 32 {
		t.Fatalf("identifiers not made and saved: %+v", saved)
	}
	if saved.Users[1].AccountID != strings.Repeat("bc", 16) || u.SubToken != strings.Repeat("a1", 16) {
		t.Fatal("existing identifiers and tokens must be kept")
	}
	if err := WriteSubscriptions(saved, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	again, _ := config.LoadFrom(path)
	if again.ServerID != saved.ServerID || again.Users[0].AccountID != u.AccountID || again.Users[0].PeriodID != u.PeriodID {
		t.Fatal("identifiers must not change once made")
	}
}

// Renewing keeps the account and its link, and starts a new period.
func TestRenewKeepsAccountAndStartsPeriod(t *testing.T) {
	arch := config.ArchivedUser{UserConfig: config.UserConfig{
		Username: "x", Password: "p", SubToken: "tok", AccountID: "acc", PeriodID: "old",
	}, Reason: "traffic limit"}
	u := renewedUser(arch, 30, 10, statusNow)
	if u.AccountID != "acc" || u.SubToken != "tok" || u.Password != "p" {
		t.Fatalf("renew must keep the account, link and password: %+v", u)
	}
	if u.PeriodID == "" || u.PeriodID == "old" {
		t.Fatalf("renew must start a new period, got %q", u.PeriodID)
	}
	if u.QuotaGB != 10 || u.ExpiresAt == "" {
		t.Fatalf("new limits: %+v", u)
	}
	fresh := newUser("y", "p", 0, 0, statusNow)
	if len(fresh.AccountID) != 32 || len(fresh.PeriodID) != 32 || fresh.AccountID == fresh.PeriodID {
		t.Fatalf("a new user gets its own identifiers: %+v", fresh)
	}
}
