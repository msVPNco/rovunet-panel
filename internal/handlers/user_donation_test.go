package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
)

func donationUserFixture(now time.Time) config.UserConfig {
	return config.UserConfig{
		Username: "gift-fixture", Password: "synthetic-secret", Donation: true,
		AccountID: strings.Repeat("da", 16), PeriodID: strings.Repeat("db", 16),
		SubToken: strings.Repeat("dc", 16), QuotaGB: donationQuotaGB,
		ExpiresAt: now.Add(donationDays * 24 * time.Hour).UTC().Format(time.RFC3339),
	}
}

func TestBuildDonationCandidatePolicyAndStandardLinks(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	u := donationUserFixture(now)
	profiles := []clientexport.Profile{
		{ID: "tls", Link: "vless://synthetic@fixture.test:443?security=tls&type=ws"},
		{ID: "reality", Link: "vless://synthetic@fixture.test:443?security=reality"},
		{ID: "ss", Link: "ss://synthetic@fixture.test:8388"},
		{ID: "trojan", Link: "trojan://synthetic@fixture.test:443"},
		{ID: "hy2", Link: "hysteria2://synthetic@fixture.test:8443"},
	}
	candidate, err := buildDonationCandidate(u, profiles, now)
	ok16(t, err)
	if candidate.Version != 1 || candidate.Kind != "rovunet-donation" || candidate.DonationID != u.AccountID || candidate.QuotaGB != 50 || u.QuotaBytes() != 50*(1<<30) || candidate.CreatedAt != now.Format(time.RFC3339) || candidate.ExpiresAt != u.ExpiresAt || len(candidate.Items) != len(profiles) {
		t.Fatal("Free config shared quota, lifetime or stable identity changed")
	}
	for i, item := range candidate.Items {
		if item.ID != profiles[i].ID || item.Link != profiles[i].Link {
			t.Fatal("candidate rewrote a profile or credential")
		}
	}
}

func TestBuildDonationCandidateRejectsIneligibleOrUnsafe(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	good := []clientexport.Profile{{ID: "vless", Link: "vless://synthetic@fixture.test:443?security=tls"}}
	for _, kind := range []string{"ordinary-account", "wrong-quota", "expired", "missing-expiry", "longer-than-30-days", "invalid-identity", "empty", "duplicate", "plain-vless", "implicit-plain-vless", "insecure-tls", "duplicate-insecure-flag", "insecure-hy2", "private-envelope", "no-login", "file-profile"} {
		t.Run(kind, func(t *testing.T) {
			u := donationUserFixture(now)
			profiles := append([]clientexport.Profile(nil), good...)
			switch kind {
			case "ordinary-account":
				u.Donation = false
			case "wrong-quota":
				u.QuotaGB = 0
			case "expired":
				u.ExpiresAt = now.Format(time.RFC3339)
			case "missing-expiry":
				u.ExpiresAt = ""
			case "longer-than-30-days":
				u.ExpiresAt = now.Add(31 * 24 * time.Hour).Format(time.RFC3339)
			case "invalid-identity":
				u.AccountID = ""
			case "empty":
				profiles = nil
			case "duplicate":
				profiles = append(profiles, profiles[0])
			case "plain-vless":
				profiles[0].Link = "vless://synthetic@fixture.test:80?security=none"
			case "implicit-plain-vless":
				profiles[0].Link = "vless://synthetic@fixture.test:80"
			case "insecure-tls":
				profiles[0].Link += "&allowInsecure=true"
			case "duplicate-insecure-flag":
				profiles[0].Link += "&allowInsecure=0&allowInsecure=1"
			case "insecure-hy2":
				profiles[0].Link = "hy2://synthetic@fixture.test:8443?insecure=1"
			case "private-envelope":
				profiles[0].Link = "rovunet://PRIVATE-FIXTURE"
			case "no-login":
				profiles[0].NoLogin = true
			case "file-profile":
				profiles[0].File = true
			}
			if _, err := buildDonationCandidate(u, profiles, now); err == nil {
				t.Fatal("unsafe or ineligible candidate was accepted")
			}
		})
	}
}

func donationExportFixture(t *testing.T) (*config.Config, *quota.Store, quota.Key) {
	t.Helper()
	isolatedSubscriptions(t)
	cfg := subTestConfig()
	u := donationUserFixture(time.Now())
	dir := filepath.Join(t.TempDir(), "ledger")
	store, err := quota.Create(context.Background(), dir, cfg.ServerID, "free-config-fixture")
	ok16(t, err)
	t.Cleanup(func() { store.Close() })
	key := quota.Key{Account: u.AccountID, Period: u.PeriodID}
	ok16(t, store.NewPeriod(context.Background(), key, u.QuotaBytes()))
	ok16(t, store.FinishBootstrap(dir))
	u.SQLiteMeter = &config.SQLiteBinding{Directory: dir, Store: "free-config-fixture"}
	cfg.AddUser(u)
	secret := filepath.Join(t.TempDir(), "vless-secret")
	ok16(t, os.WriteFile(secret, []byte(strings.Repeat("17", 32)), 0600))
	cfg.Tunnels = append(cfg.Tunnels,
		config.TunnelConfig{Tag: "vless", Enabled: true, Transport: config.TransportVLESS, Domain: "fixture.test", VLESS: &config.VLESSConfig{Port: 443, Path: "/ws", Front: "naive", SecretFile: secret, TLS: true, CDN: true}},
		config.TunnelConfig{Tag: "direct-ssh", Enabled: true, Transport: config.TransportSSH, Backend: config.BackendSSH},
		config.TunnelConfig{Tag: "slipstream-socks", Enabled: true, Transport: config.TransportSlipstream, Backend: config.BackendSOCKS},
	)
	ok16(t, cfg.SaveTo(filepath.Join(t.TempDir(), "config.json")))
	ok16(t, WriteSubscriptions(cfg, discardOutput{}))
	return cfg, store, key
}

func TestDonationExportPrivateMeteredOnlyAndPreservesAccounts(t *testing.T) {
	cfg, _, _ := donationExportFixture(t)
	before, err := os.ReadFile(cfg.Path())
	ok16(t, err)
	oldUser := cfg.Users[0]
	oldAliases := aliases16(t, oldUser.SubToken)
	ok16(t, writeDonationCandidate(cfg, "gift-fixture", discardOutput{}))
	path := filepath.Join(filepath.Dir(cfg.Path()), "donations", "gift-fixture.json")
	body, err := os.ReadFile(path)
	ok16(t, err)
	var candidate donationCandidate
	ok16(t, json.Unmarshal(body, &candidate))
	if len(candidate.Items) != 1 || candidate.Items[0].ID != "vless" || strings.Contains(string(body), oldUser.Password) || strings.Contains(string(body), cfg.Users[1].Password) {
		t.Fatal("Free config exported an unmetered path or another account's credentials")
	}
	for path, mode := range map[string]os.FileMode{path: 0600, filepath.Dir(path): 0700} {
		info, err := os.Stat(path)
		ok16(t, err)
		if info.Mode().Perm() != mode {
			t.Fatal("candidate is not private", info.Mode())
		}
	}
	after, err := os.ReadFile(cfg.Path())
	ok16(t, err)
	if !bytes.Equal(before, after) || cfg.Users[0] != oldUser || cfg.SubscriptionV2 {
		t.Fatal("export changed config, account identity or v2 opt-in")
	}
	same16(t, oldAliases, aliases16(t, oldUser.SubToken))
	ok16(t, writeDonationCandidate(cfg, "gift-fixture", discardOutput{}))
	again, err := os.ReadFile(path)
	ok16(t, err)
	if !bytes.Equal(body, again) {
		t.Fatal("repeat export changed stable candidate identity")
	}
}

func TestDonationExportRejectsUnavailableExhaustedOrPendingQuota(t *testing.T) {
	for _, kind := range []string{"unbound", "exhausted", "fully-pending", "closed-period", "lost-ledger", "recovery-hold"} {
		t.Run(kind, func(t *testing.T) {
			cfg, store, key := donationExportFixture(t)
			ok16(t, writeDonationCandidate(cfg, "gift-fixture", discardOutput{}))
			path := filepath.Join(filepath.Dir(cfg.Path()), "donations", "gift-fixture.json")
			before, err := os.ReadFile(path)
			ok16(t, err)
			u := cfg.GetUser("gift-fixture")
			switch kind {
			case "unbound":
				u.SQLiteMeter = nil
			case "exhausted", "fully-pending":
				ok16(t, store.ReserveNative(context.Background(), key, "full-budget", "up", u.QuotaBytes()))
				if kind == "exhausted" {
					ok16(t, store.Settle(context.Background(), key, "full-budget", u.QuotaBytes()))
				}
			case "closed-period":
				ok16(t, store.Renew(context.Background(), key, "renew-fixture", strings.Repeat("ee", 16), u.QuotaBytes()))
			case "lost-ledger":
				ok16(t, os.Rename(u.SQLiteMeter.Directory, u.SQLiteMeter.Directory+".retained"))
			case "recovery-hold":
				cfg.RecoverySealed = true
			}
			if err := writeDonationCandidate(cfg, "gift-fixture", discardOutput{}); err == nil {
				t.Fatal("unavailable or consumed budget was exported")
			}
			after, err := os.ReadFile(path)
			ok16(t, err)
			if !bytes.Equal(before, after) {
				t.Fatal("failed export replaced the previous complete private candidate")
			}
		})
	}
}

func TestDonationExportRejectsSymlinks(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			cfg, _, _ := donationExportFixture(t)
			dir := filepath.Join(filepath.Dir(cfg.Path()), "donations")
			outside := filepath.Join(t.TempDir(), "outside")
			if directory {
				ok16(t, os.Mkdir(outside, 0700))
				ok16(t, os.Symlink(outside, dir))
			} else {
				ok16(t, os.Mkdir(dir, 0700))
				ok16(t, os.WriteFile(outside, []byte("must remain unchanged"), 0600))
				ok16(t, os.Symlink(outside, filepath.Join(dir, "gift-fixture.json")))
			}
			if err := writeDonationCandidate(cfg, "gift-fixture", discardOutput{}); err == nil {
				t.Fatal("export followed a symlink")
			}
			if directory {
				entries, err := os.ReadDir(outside)
				ok16(t, err)
				if len(entries) != 0 {
					t.Fatal("credentials escaped into the linked directory")
				}
			} else {
				body, err := os.ReadFile(outside)
				ok16(t, err)
				if string(body) != "must remain unchanged" {
					t.Fatal("export overwrote a linked destination")
				}
			}
		})
	}
}

func TestDonationCreationFailureKeepsMarkedAccountAndOldURLs(t *testing.T) {
	isolatedSubscriptions(t)
	accounts := fakeSSHAccounts(t)
	cfg := subTestConfig()
	ok16(t, cfg.SaveTo(filepath.Join(t.TempDir(), "config.json")))
	ok16(t, WriteSubscriptions(cfg, discardOutput{}))
	oldUser := cfg.Users[0]
	oldAliases := aliases16(t, oldUser.SubToken)
	oldFinish := finishNewUserAccounting
	t.Cleanup(func() { finishNewUserAccounting = oldFinish })
	failure := errors.New("synthetic accounting failure")
	finishNewUserAccounting = func(c *config.Config, fresh map[string]bool) error {
		saved, err := config.LoadFrom(c.Path())
		ok16(t, err)
		u := saved.GetUser("gift-partial")
		if u == nil || !u.Donation || u.QuotaBytes() != 50*(1<<30) || !fresh[u.AccountID] {
			t.Fatal("first account save lost the Free config marker or finite policy")
		}
		return failure
	}
	start := time.Now().UTC().Truncate(time.Second)
	if err := createDonationUser(cfg, discardOutput{}, "gift-partial"); !errors.Is(err, failure) {
		t.Fatal("creation did not report incomplete accounting", err)
	}
	saved, err := config.LoadFrom(cfg.Path())
	ok16(t, err)
	u := saved.GetUser("gift-partial")
	expiry, err := time.Parse(time.RFC3339, u.ExpiresAt)
	ok16(t, err)
	if !u.Donation || u.Password == "" || expiry.Before(start.Add(30*24*time.Hour)) || expiry.After(time.Now().Add(30*24*time.Hour)) || saved.Users[0] != oldUser || len(accounts.added) != 0 || len(accounts.removed) != 0 {
		t.Fatal("partial creation changed policy, existing account, or SSH access")
	}
	same16(t, oldAliases, aliases16(t, oldUser.SubToken))
	if err := writeDonationCandidate(saved, u.Username, discardOutput{}); err == nil {
		t.Fatal("partially provisioned account was exported")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfg.Path()), "donations")); !os.IsNotExist(err) {
		t.Fatal("failed creation published a candidate")
	}
	before := *u
	if err := createDonationUser(saved, discardOutput{}, u.Username); err == nil {
		t.Fatal("retry silently replaced an existing account")
	}
	if *saved.GetUser(u.Username) != before {
		t.Fatal("retry reset credentials, quota or expiry")
	}
}

func TestDonationCannotConvertExistingOrArchivedUsers(t *testing.T) {
	for _, archived := range []bool{false, true} {
		cfg := subTestConfig()
		before := cfg.Users[0]
		if archived {
			cfg.Archived = []config.ArchivedUser{{UserConfig: before, Reason: "expired"}}
			cfg.Users = cfg.Users[1:]
		}
		if err := createDonationUser(cfg, discardOutput{}, before.Username); err == nil {
			t.Fatal("existing normal account converted into Free config")
		}
		if archived && cfg.Archived[0].UserConfig != before || !archived && cfg.Users[0] != before {
			t.Fatal("normal account identity changed")
		}
	}
}

func TestDonationMenuActionsAreRegistered(t *testing.T) {
	action, ok := actions.Get(actions.SystemUsers)
	if !ok {
		t.Fatal("Users action is missing")
	}
	choices := map[string]string{}
	dependencies := map[string]bool{}
	for _, input := range action.Inputs {
		if input.Key == "action" {
			for _, option := range input.Options {
				choices[option.Value] = option.Label
			}
		}
		if input.Key == "username" {
			for _, value := range input.DependsOnValues {
				dependencies[value] = true
			}
		}
	}
	if choices["donate"] != "Free config (50 GB / 30 days)" || choices["donation-export"] != "Export Free config for review" || !dependencies["donate"] || !dependencies["donation-export"] {
		t.Fatal("registered Users menu cannot create/export a named Free config")
	}
}
