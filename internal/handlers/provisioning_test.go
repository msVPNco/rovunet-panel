package handlers

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

func TestFinalProvisioningIncludesBothSoarModesBeforeSubscription(t *testing.T) {
	old := transport.SubscriptionDir
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subs")
	t.Cleanup(func() { transport.SubscriptionDir = old })
	cfg := subTestConfig()
	u := cfg.Users[0]
	account, period, token := u.AccountID, u.PeriodID, u.SubToken
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{config.BackendDirect, config.BackendSSH} {
		tag := "soar"
		if backend == config.BackendSSH {
			tag = "soar-ssh"
		}
		cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{Tag: tag, Enabled: true, Transport: config.TransportSoar, Backend: backend, Domain: tag + ".example.test", Soar: &config.SoarConfig{PublicKey: strings.Repeat("ab", 32)}})
	}
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	b, err := buildFeed(cfg, u)
	if err != nil {
		t.Fatal(err)
	}
	var feed subFeed
	if err := json.Unmarshal(b, &feed); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, it := range feed.Items {
		ids[it.ID] = true
	}
	if !ids["soar"] || !ids["soar-ssh"] {
		t.Fatal("final subscription missed a Soar mode")
	}
	out := &printOutput{}
	printClientConfigs(cfg, out, cfg.Tunnels)
	soar, sub := -1, -1
	for i, line := range out.lines {
		if strings.Contains(line, "[soar] alice") {
			soar = i
		}
		if strings.Contains(line, SubscriptionURL(cfg, u)) {
			sub = i
			break
		}
	}
	if soar < 0 || sub < soar {
		t.Fatal("subscription was printed before this user's complete profile set")
	}
	if cfg.Users[0].AccountID != account || cfg.Users[0].PeriodID != period || cfg.Users[0].SubToken != token {
		t.Fatal("subscription refresh changed account or billing period identity")
	}
	u.QuotaGB = 1
	b, err = buildFeed(cfg, u)
	if err != nil || b != nil {
		t.Fatal("uncounted Soar paths appeared for a quota account")
	}
}
