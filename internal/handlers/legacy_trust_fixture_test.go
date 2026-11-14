package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

func TestLegacyTrustBothSlipstreamBackendsWithoutV2(t *testing.T) {
	c, certPath, certificate := trustFixture25(t)
	block, _ := pem.Decode(certificate)
	if block == nil {
		t.Fatal("synthetic certificate is not PEM")
	}
	derHash := sha256.Sum256(block.Bytes)
	c.SubscriptionV2 = false
	u := c.Users[0]
	c.Tunnels = append(c.Tunnels, config.TunnelConfig{
		Tag: "slipstream-ssh", Transport: config.TransportSlipstream,
		Backend: config.BackendSSH, Domain: "ssh-slip.example.test", Enabled: true,
		Slipstream: &config.SlipstreamConfig{Cert: certPath, Key: "DO-NOT-READ-PRIVATE-KEY"},
	})
	before, err := buildFeed(c, u)
	ok16(t, err)
	body, err := buildSubscriptionTrust(c, u)
	ok16(t, err)
	var trust subscriptionTrust
	ok16(t, json.Unmarshal(body, &trust))
	if trust.Version != 1 || trust.Server != c.ServerID || trust.Account != u.AccountID || trust.Period != u.PeriodID || len(trust.Items) != 2 {
		t.Fatal("account binding or backend coverage changed")
	}
	want := map[string]string{"slipstream-socks": "slip.example.test", "slipstream-ssh": "ssh-slip.example.test"}
	for _, item := range trust.Items {
		if item.Domain != want[item.Profile] || item.Transport != "slipstream" || item.SHA256 != hex.EncodeToString(derHash[:]) || item.Certificate != string(certificate) {
			t.Fatal("profile/domain/certificate binding changed")
		}
		delete(want, item.Profile)
	}
	if len(want) != 0 || strings.Contains(string(body), "PRIVATE KEY") || strings.Contains(string(body), "DO-NOT-READ") {
		t.Fatal("missing backend or private material exported")
	}
	after, err := buildFeed(c, u)
	ok16(t, err)
	if string(before) != string(after) || c.SubscriptionV2 {
		t.Fatal("feed or feature flag changed")
	}
	if dir := os.Getenv("ROVUNET_LEGACY_TRUST_ROUTE_FIXTURE"); dir != "" {
		old := transport.SubscriptionDir
		transport.SubscriptionDir = filepath.Join(dir, "subscriptions")
		defer func() { transport.SubscriptionDir = old }()
		ok16(t, WriteSubscriptions(c, discardOutput{}))
		// Keep synthetic token private to the local fixture; never a customer URL.
		ok16(t, os.WriteFile(filepath.Join(dir, "expected.json"), body, 0600))
		ok16(t, os.WriteFile(filepath.Join(dir, "token.txt"), []byte(u.SubToken), 0600))
	}
}

func TestTrustFiniteAndExpiredAccountsDoNotReceiveSlipstreamItems(t *testing.T) {
	c, certPath, _ := trustFixture25(t)
	c.Tunnels = append(c.Tunnels, config.TunnelConfig{Tag: "slipstream-ssh", Transport: config.TransportSlipstream, Backend: config.BackendSSH, Domain: "ssh-slip.example.test", Enabled: true, Slipstream: &config.SlipstreamConfig{Cert: certPath, Key: "DO-NOT-READ"}})
	u := c.Users[0]
	u.QuotaGB = 1
	body, err := buildSubscriptionTrust(c, u)
	ok16(t, err)
	var trust subscriptionTrust
	ok16(t, json.Unmarshal(body, &trust))
	if len(trust.Items) != 0 {
		t.Fatal("finite account received an unaccounted Slipstream identity")
	}
	old := transport.SubscriptionDir
	transport.SubscriptionDir = t.TempDir()
	defer func() { transport.SubscriptionDir = old }()
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	path := filepath.Join(publicDir(), c.Users[0].SubToken+trustSuffix)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	c.Users[0].ExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expired account retained a public trust resource")
	}
}
