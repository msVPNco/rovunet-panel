package handlers

import (
	"github.com/msVPNco/rovunet-panel/internal/keys"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestParseLimit(t *testing.T) {
	for _, s := range []string{"", " ", "0", "free", "Unlimited"} {
		if n, err := parseLimit(s, 100); err != nil || n != 0 {
			t.Errorf("%q: got %d, %v; want unlimited", s, n, err)
		}
	}
	if n, err := parseLimit("30", 100); err != nil || n != 30 {
		t.Errorf("30: got %d, %v", n, err)
	}
	for _, s := range []string{"-1", "abc", "101", "1.5"} {
		if _, err := parseLimit(s, 100); err == nil {
			t.Errorf("%q must be rejected", s)
		}
	}
}

func TestAskUserLimitsFromArguments(t *testing.T) {
	days, gb, err := askUserLimits("30", "50")
	if err != nil || days != 30 || gb != 50 {
		t.Fatalf("got %d %d %v", days, gb, err)
	}
	days, gb, err = askUserLimits("", "20")
	if err != nil || days != 0 || gb != 20 {
		t.Fatalf("gb only: got %d %d %v", days, gb, err)
	}
	if _, _, err := askUserLimits("x", ""); err == nil || !strings.Contains(err.Error(), "days") {
		t.Fatalf("bad days: %v", err)
	}
}

func TestNewUserExpiry(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	free := newUser("a", "p", 0, 0, now)
	if free.ExpiresAt != "" || free.QuotaGB != 0 || free.Expired(now.Add(1000*time.Hour)) {
		t.Fatalf("unlimited user must never expire: %+v", free)
	}
	if got := limitSummary(free); got != "unlimited" {
		t.Fatalf("summary %q", got)
	}
	u := newUser("b", "p", 30, 20, now)
	if u.ExpiresAt != "2026-12-07T12:00:00Z" || u.QuotaGB != 20 {
		t.Fatalf("got %+v", u)
	}
	if u.Expired(now.Add(29*24*time.Hour)) || !u.Expired(now.Add(30*24*time.Hour)) {
		t.Fatal("expiry must fall exactly 30 days later")
	}
	if got := limitSummary(u); got != "expires 2026-12-07 12:00 UTC, 20 GB" {
		t.Fatalf("summary %q", got)
	}
}

func TestUnreadableExpiryNeverDeletes(t *testing.T) {
	u := config.UserConfig{Username: "x", ExpiresAt: "garbage"}
	if u.Expired(time.Now().Add(100000 * time.Hour)) {
		t.Fatal("an unreadable date must not expire a user")
	}
}

func TestOverQuota(t *testing.T) {
	gb := int64(1 << 30)
	u := config.UserConfig{Username: "a", QuotaGB: 2, UsageBase: 5 * gb}
	if overQuota(u, map[string]int64{"a": 6 * gb}) {
		t.Fatal("1 GB of 2 GB used since the base is under the limit")
	}
	if !overQuota(u, map[string]int64{"a": 7 * gb}) {
		t.Fatal("2 GB used since the base reaches the limit")
	}
	if overQuota(config.UserConfig{Username: "b"}, map[string]int64{"b": 1000 * gb}) {
		t.Fatal("a user without a limit is never over it")
	}
	if got := UsedBytes(u, map[string]int64{"a": 4 * gb}); got != 0 {
		t.Fatalf("totals below the base must read as zero, got %d", got)
	}
}

// A traffic-limited user is given only VLESS; an unlimited one gets all.
func TestLimitedUserGetsOnlyVLESS(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "vless.secret")
	if err := keys.GenerateVLESSSecret(secret); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		PublicAddress: "203.0.113.7",
		Backends:      config.DefaultBackends(),
		Users: []config.UserConfig{
			{Username: "free", Password: "pw-free"},
			{Username: "capped", Password: "pw-capped", QuotaGB: 5},
		},
		Tunnels: []config.TunnelConfig{
			{Tag: "vless", Transport: config.TransportVLESS, Backend: config.BackendDirect, Enabled: true, Domain: "cdn.example.com",
				VLESS: &config.VLESSConfig{Port: 2053, Path: "/p", TLS: true, Cert: "c", Key: "k", SecretFile: secret, CDN: true}},
			{Tag: "naive", Transport: config.TransportNaive, Backend: config.BackendSOCKS, Enabled: true, Domain: "n.example.com", Naive: &config.NaiveConfig{Port: 443}},
			{Tag: "soar", Transport: config.TransportSoar, Backend: config.BackendDirect, Enabled: true, Domain: "r.example.com",
				Soar: &config.SoarConfig{PublicKey: strings.Repeat("ab", 32)}},
		},
	}
	free, capped := &printOutput{}, &printOutput{}
	printUserConfigs(cfg, "free", "pw-free", free)
	printUserConfigs(cfg, "capped", "pw-capped", capped)
	if free.count("[naive-socks]") != 1 || free.count("[soar]") == 0 || free.count("[vless]") == 0 {
		t.Fatalf("unlimited user must get everything:\n%s", strings.Join(free.lines, "\n"))
	}
	if capped.count("[vless]") == 0 || capped.count("[naive") != 0 || capped.count("[soar") != 0 || capped.count("pw-capped") != 0 {
		t.Fatalf("limited user must get only VLESS:\n%s", strings.Join(capped.lines, "\n"))
	}

	// The all-configs view and the shared Soar config follow the same rule.
	all := &printOutput{}
	printClientConfigs(cfg, all, cfg.Tunnels)
	if all.count("[soar] capped") != 0 || all.count("[naive-socks] capped") != 0 || all.count("[vless] capped") == 0 {
		t.Fatalf("all-configs view:\n%s", strings.Join(all.lines, "\n"))
	}
	if all.count("[soar] free, no login needed") != 1 {
		t.Fatalf("unlimited user keeps Soar:\n%s", strings.Join(all.lines, "\n"))
	}
}

func TestArchivedUserRoundTrip(t *testing.T) {
	cfg := &config.Config{Archived: []config.ArchivedUser{{
		UserConfig: config.UserConfig{Username: "x", Password: "p", SubToken: "t", QuotaGB: 3},
		Reason:     "traffic limit", At: "2026-11-07T12:00:00Z",
	}}}
	if !forgetArchived(cfg, "x") || len(cfg.Archived) != 0 || forgetArchived(cfg, "x") {
		t.Fatal("forgetArchived must drop the record once")
	}
}
