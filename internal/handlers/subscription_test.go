package handlers

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

func subTestConfig() *config.Config {
	// Every identifier is set: a config made in a test has no file to save
	// minted ones to.
	return &config.Config{
		PublicAddress: "203.0.113.7",
		ServerID:      strings.Repeat("5e", 16),
		Backends:      config.DefaultBackends(),
		Users: []config.UserConfig{
			{Username: "alice", Password: "pw-alice", SubToken: strings.Repeat("a1", 16), AccountID: strings.Repeat("ac", 16), PeriodID: strings.Repeat("9a", 16)},
			{Username: "bob", Password: "pw-bob", SubToken: strings.Repeat("b2", 16), AccountID: strings.Repeat("bc", 16), PeriodID: strings.Repeat("9b", 16)},
		},
		Tunnels: []config.TunnelConfig{{
			Tag: "naive", Transport: config.TransportNaive, Backend: config.BackendSOCKS, Enabled: true,
			Domain: "n.example.com", Naive: &config.NaiveConfig{Port: 443},
		}},
	}
}

func TestStructuredSubscriptionItems(t *testing.T) {
	cfg := subTestConfig()
	items, err := userSubscriptionItems(cfg, cfg.Users[0])
	if err != nil || len(items) != 2 || items[0].ID != "naive-socks" || items[1].ID != "naive-ssh" {
		t.Fatalf("structured feed: count=%d err=%v", len(items), err)
	}
}
func TestSubscriptionURL(t *testing.T) {
	cfg := subTestConfig()
	got := SubscriptionURL(cfg, cfg.Users[0])
	if got != "https://n.example.com/sub/"+cfg.Users[0].SubToken+".json" {
		t.Fatalf("got %s", got)
	}
	cfg.Tunnels[0].Naive.Port = 8443
	if got := SubscriptionURL(cfg, cfg.Users[0]); !strings.Contains(got, "n.example.com:8443/") {
		t.Fatalf("port missing: %s", got)
	}
	cfg.Tunnels = nil
	if got := SubscriptionURL(cfg, cfg.Users[0]); got != "" {
		t.Fatalf("no NaiveProxy, no link, got %s", got)
	}
}

// The feed has exactly the keys the app accepts, one item per config.
func TestWriteSubscriptionsFeedsAndCleanup(t *testing.T) {
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subs")
	cfg := subTestConfig()
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(transport.SubscriptionDir, "sub")
	raw, err := os.ReadFile(filepath.Join(dir, cfg.Users[0].SubToken+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	for k := range top {
		if k != "version" && k != "items" && k != "name" {
			t.Fatalf("the app rejects key %q", k)
		}
	}
	var feed subFeed
	if err := json.Unmarshal(raw, &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Version != 1 || feed.Name != "alice" || len(feed.Items) != 2 {
		t.Fatalf("feed %+v", feed)
	}
	ids := map[string]bool{}
	for _, it := range feed.Items {
		if !strings.HasPrefix(it.Link, "rovunet://") || ids[it.ID] || it.ID == "" || len(it.ID) > 200 {
			t.Fatalf("bad item %+v", it)
		}
		ids[it.ID] = true
	}
	if !ids["naive-socks"] || !ids["naive-ssh"] {
		t.Fatalf("want both naive configs, got %v", ids)
	}
	if strings.Contains(string(raw), "pw-bob") {
		t.Fatal("a feed must not carry another user's credentials")
	}

	// A removed user's file goes away; a changed token retires the old file.
	cfg.Users = cfg.Users[:1]
	cfg.Users[0].SubToken = strings.Repeat("c3", 16)
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{cfg.Users[0].SubToken + ".json", cfg.Users[0].SubToken + ".status.json", cfg.Users[0].SubToken + trustSuffix, cfg.Users[0].SubToken + ".windows.json", cfg.Users[0].SubToken + ".windows.status.json"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("got files %v, want %v", names, want)
	}
}

func TestWriteSubscriptionsWithoutNaiveDoesNothing(t *testing.T) {
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subs")
	cfg := subTestConfig()
	cfg.Tunnels = nil
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(transport.SubscriptionDir); err == nil {
		t.Fatal("no directory should be created without a NaïveProxy tunnel")
	}
}

// `users configs` for everyone (the install summary uses the same printer)
// shows each user's link; a user from before links existed gets one.
func TestAllConfigsPrintSubscriptionLinks(t *testing.T) {
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subs")
	cfg := subTestConfig()
	cfg.Users[1].SubToken = "" // made before links existed
	path := filepath.Join(t.TempDir(), "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(path) // a config with a save path, as in real use
	if err != nil {
		t.Fatal(err)
	}
	out := &printOutput{}
	printClientConfigs(cfg, out, cfg.Tunnels)
	if out.count("https://n.example.com/sub/") != 4 {
		t.Fatalf("want a link per user:\n%s", strings.Join(out.lines, "\n"))
	}
	if cfg.Users[1].SubToken == "" {
		t.Fatal("the missing token must have been created")
	}
}

// Links that differ only in their group name are the same link.
func TestLinkIdentityIgnoresGroupFields(t *testing.T) {
	mk := func(host, group string) string {
		f := make([]string, clientcfg.FCollectionName+1)
		f[3] = host
		f[clientcfg.FCollectionID] = group
		f[clientcfg.FCollectionName] = group
		return "rovunet://" + base64.StdEncoding.EncodeToString([]byte(strings.Join(f, "|")))
	}
	if linkIdentity(mk("h", "a")) != linkIdentity(mk("h", "b")) {
		t.Fatal("group fields should not matter")
	}
	if linkIdentity(mk("h", "a")) == linkIdentity(mk("other", "a")) {
		t.Fatal("a different host should matter")
	}
}

// Address-based configs are repeated for each extra address; domain-based
// ones (NaïveProxy) are not, since they would be identical.
func TestFeedRepeatsAddressConfigsPerExtraAddress(t *testing.T) {
	cfg := subTestConfig()
	cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
		Tag: "ssh", Transport: config.TransportSSH, Backend: config.BackendSSH, Enabled: true,
	})
	base, err := buildFeed(cfg, cfg.Users[0])
	if err != nil {
		t.Fatal(err)
	}
	var before subFeed
	if err := json.Unmarshal(base, &before); err != nil {
		t.Fatal(err)
	}

	cfg.ExtraAddresses = []string{"198.51.100.9"}
	raw, err := buildFeed(cfg, cfg.Users[0])
	if err != nil {
		t.Fatal(err)
	}
	var after subFeed
	if err := json.Unmarshal(raw, &after); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, it := range after.Items {
		if ids[it.ID] {
			t.Fatalf("duplicate id %q", it.ID)
		}
		ids[it.ID] = true
	}
	extra := 0
	for id := range ids {
		if strings.HasSuffix(id, "-ip2") {
			extra++
			if strings.HasPrefix(id, "naive") {
				t.Fatalf("a domain-based config was repeated: %s", id)
			}
		}
	}
	if extra == 0 || len(after.Items) != len(before.Items)+extra {
		t.Fatalf("before %d items, after %d, %d extra", len(before.Items), len(after.Items), extra)
	}
}
