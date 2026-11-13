package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/keys"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

func isolatedSubscriptions(t *testing.T) {
	t.Helper()
	oldSubs, oldCores, oldUsage := transport.SubscriptionDir, coreinstall.Root, usage.Dir
	t.Cleanup(func() { transport.SubscriptionDir, coreinstall.Root, usage.Dir = oldSubs, oldCores, oldUsage })
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subscriptions")
	coreinstall.Root, usage.Dir = t.TempDir(), t.TempDir()
}

// Store confirmed per-account files, without installing units or binding ports.
func subscriptionCoreFixture(t *testing.T, name string, options coreplan.Options, user config.UserConfig) {
	t.Helper()
	options.CoreName = name
	state, err := coreplan.NewState()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := coreplan.GenerateFor(options, &state, []coreplan.Client{{Name: user.Username, Secret: user.Password}})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(coreinstall.Root, name)
	if err := coreinstall.WriteBundle(dir, options, bundle); err != nil {
		t.Fatal(err)
	}
	clientName := user.Username + "." + bundle.Extension
	if err := os.Rename(filepath.Join(dir, "clients", "device-001."+bundle.Extension), filepath.Join(dir, "clients", clientName)); err != nil {
		t.Fatal(err)
	}
	applied := map[string]any{"server": json.RawMessage(bundle.Server), "clients": map[string]string{clientName: bundle.Clients[0] + "\n"}}
	for name, value := range map[string]any{"options.json": options, "state.json": state, "applied.json": applied} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func savedSubscription(t *testing.T, user config.UserConfig, client string) subFeed {
	t.Helper()
	suffix := ".json"
	if client == "windows" {
		suffix = ".windows.json"
	}
	data, err := os.ReadFile(filepath.Join(transport.SubscriptionDir, "sub", user.SubToken+suffix))
	if err != nil {
		t.Fatal(err)
	}
	var feed subFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	return feed
}

func twentyProfileSubscriptionFixture(t *testing.T) (*config.Config, string) {
	t.Helper()
	isolatedSubscriptions(t)
	cfg := subTestConfig()
	cfg.Users = cfg.Users[:1]
	cfg.Tunnels = append(cfg.Tunnels, configsTestTunnels()...)
	for i := range cfg.Tunnels {
		if cfg.Tunnels[i].Tag == "dnstt" {
			cfg.Tunnels[i].Tag = "dnstt-socks"
		}
	}
	cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{Tag: "dnstt-ssh", Enabled: true, Transport: config.TransportDNSTT, Backend: config.BackendSSH, Domain: "dns.example.test", DNSTT: &config.DNSTTConfig{PublicKey: strings.Repeat("bc", 32)}})
	for _, protocol := range []string{config.TransportSlipstream, config.TransportVayDNS} {
		for _, backend := range []string{config.BackendSOCKS, config.BackendSSH} {
			cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{Tag: protocol + "-" + backend, Enabled: true, Transport: protocol, Backend: backend, Domain: protocol + ".example.test"})
		}
	}
	secret := filepath.Join(t.TempDir(), "vless.secret")
	if err := keys.GenerateVLESSSecret(secret); err != nil {
		t.Fatal(err)
	}
	cfg.Tunnels = append(cfg.Tunnels,
		config.TunnelConfig{Tag: "direct-ssh", Enabled: true, Transport: config.TransportSSH, Backend: config.BackendSSH},
		config.TunnelConfig{Tag: "stuntls", Enabled: true, Transport: config.TransportStunTLS, Backend: config.BackendSSH, Domain: "tls.example.test", StunTLS: &config.StunTLSConfig{Port: 443}},
		config.TunnelConfig{Tag: "vless", Enabled: true, Transport: config.TransportVLESS, Backend: config.BackendDirect, Domain: "cdn.example.test", VLESS: &config.VLESSConfig{Port: 443, Path: "/ws", TLS: true, CDN: true, Cert: "cert", Key: "key", SecretFile: secret}},
		config.TunnelConfig{Tag: "socks", Enabled: true, Transport: config.TransportSOCKS, Backend: config.BackendSOCKS},
	)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	user := cfg.Users[0]
	for i, name := range []string{"reality", "reality2", "xh", "hy2", "wireguard"} {
		protocol := name
		switch name {
		case "reality2":
			protocol = "reality"
		case "xh":
			protocol = "xhttp"
		case "hy2":
			protocol = "hysteria2"
		}
		subscriptionCoreFixture(t, name, coreplan.Options{Protocol: protocol, Host: cfg.PublicAddress, SNI: "example.test", Port: 18443 + i, Cert: "cert", Key: "key", RealityTarget: "www.example.com:443"}, user)
	}
	return cfg, path
}

func addManagedSubscriptionFixture(t *testing.T, cfg *config.Config) {
	t.Helper()
	for i, protocol := range []string{"amneziawg", "ss2022", "trojan", "vlessenc"} {
		subscriptionCoreFixture(t, "managed-"+map[string]string{"amneziawg": "awg", "ss2022": "ss2022", "trojan": "trojan", "vlessenc": "vlessenc"}[protocol], coreplan.Options{Protocol: protocol, Host: cfg.PublicAddress, SNI: "example.test", Port: 28444 + i, Cert: "cert", Key: "key", Subnet: "10.203.72.0/24", Egress: "eth0"}, cfg.Users[0])
	}
}

func TestSubscriptionUpgradeKeepsExistingURLAndTwentyProfiles(t *testing.T) {
	cfg, path := twentyProfileSubscriptionFixture(t)
	var err error
	user := cfg.Users[0]
	serverID, androidURL, windowsURL := cfg.ServerID, SubscriptionURL(cfg, user), WindowsSubscriptionURL(cfg, user)
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	before := savedSubscription(t, user, "android")
	if len(before.Items) != 20 {
		t.Fatalf("baseline profiles=%d, want 20", len(before.Items))
	}
	addManagedSubscriptionFixture(t, cfg)
	for refresh := 0; refresh < 2; refresh++ {
		if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
			t.Fatal(err)
		}
		after := savedSubscription(t, user, "android")
		if len(after.Items) != 24 {
			t.Fatalf("updated profiles=%d, want 24", len(after.Items))
		}
		members := map[string]string{}
		for _, item := range after.Items {
			members[item.ID] = item.Link
		}
		for _, item := range before.Items {
			if members[item.ID] != item.Link {
				t.Fatalf("existing profile %s changed identity or credentials", item.ID)
			}
		}
		for _, id := range []string{"managed-awg", "managed-ss2022", "managed-trojan", "managed-vlessenc"} {
			if members[id] == "" {
				t.Fatalf("missing new profile %s", id)
			}
		}
		if got := len(savedSubscription(t, user, "windows").Items); got != 24 {
			t.Fatalf("Windows profiles=%d, want 24", got)
		}
		cfg, err = config.LoadFrom(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ServerID != serverID || cfg.Users[0] != user || SubscriptionURL(cfg, user) != androidURL || WindowsSubscriptionURL(cfg, user) != windowsURL {
			t.Fatal("refresh changed an existing account, period, password or URL")
		}
	}
}

func TestFailedRefreshKeepsExistingFeedAndStillRemovesDeletedAccounts(t *testing.T) {
	isolatedSubscriptions(t)
	cfg := subTestConfig()
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	alice, bob := cfg.Users[0], cfg.Users[1]
	feedPath := filepath.Join(transport.SubscriptionDir, "sub", alice.SubToken+".json")
	before, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(coreinstall.Root, "broken")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"meta": "protocol=trojan\nport=30444\nproto=tcp\n", "options.json": `{"protocol":"trojan","host":"192.0.2.1","port":30444}`, "state.json": "{broken"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Users = cfg.Users[:1]
	if err := WriteSubscriptions(cfg, discardOutput{}); err == nil {
		t.Fatal("invalid provisioning state was reported as a successful refresh")
	}
	after, err := os.ReadFile(feedPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("failed refresh deleted or replaced an existing valid feed")
	}
	for _, suffix := range []string{".json", ".windows.json", ".status.json", ".windows.status.json"} {
		if _, err := os.Stat(filepath.Join(transport.SubscriptionDir, "sub", bob.SubToken+suffix)); !os.IsNotExist(err) {
			t.Fatal("deleted account retained a subscription or status file")
		}
	}
}

func TestConcurrentRefreshKeepsValidSubscriptionFiles(t *testing.T) {
	isolatedSubscriptions(t)
	cfg := subTestConfig()
	cfg.ExtraAddresses = []string{"198.51.100.9"}
	var workers sync.WaitGroup
	failures := make(chan error, 4)
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); failures <- WriteSubscriptions(cfg, discardOutput{}) }()
	}
	workers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(savedSubscription(t, cfg.Users[0], "android").Items) != 2 {
		t.Fatal("concurrent refresh corrupted the feed")
	}
}

func TestTunnelLifecycleRefreshesTheSameSubscription(t *testing.T) {
	isolatedSubscriptions(t)
	oldStart, oldStop := startTunnelService, stopTunnelService
	t.Cleanup(func() { startTunnelService, stopTunnelService = oldStart, oldStop })
	var failStart bool
	startTunnelService = func(name string) error {
		if name != "rovunet-ssh" {
			return fmt.Errorf("unexpected service")
		}
		if failStart {
			return errors.New("start failed")
		}
		return nil
	}
	stopTunnelService = func(string) error { return nil }
	cfg := subTestConfig()
	cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{Tag: "ssh", Enabled: true, Transport: config.TransportSSH, Backend: config.BackendSSH})
	path := filepath.Join(t.TempDir(), "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	user := cfg.Users[0]
	originalURL := SubscriptionURL(cfg, user)
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	ctx := &actions.Context{Config: cfg, Args: map[string]string{"tag": "ssh"}, Output: discardOutput{}}
	if err := handleTunnelStop(ctx); err != nil {
		t.Fatal(err)
	}
	if len(savedSubscription(t, user, "android").Items) != 2 || cfg.GetTunnel("ssh").Enabled {
		t.Fatal("stopped tunnel remained in subscriptions")
	}
	failStart = true
	if handleTunnelStart(ctx) == nil || len(savedSubscription(t, user, "android").Items) != 2 {
		t.Fatal("failed start was advertised")
	}
	failStart = false
	if err := handleTunnelStart(ctx); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(savedSubscription(t, user, "android").Items) != 3 || !loaded.GetTunnel("ssh").Enabled || SubscriptionURL(loaded, user) != originalURL || loaded.Users[0] != user {
		t.Fatal("restart did not restore the same subscription and account")
	}
}

func TestQuotaNoticeDoesNotCountExcludedCoresAsUsable(t *testing.T) {
	isolatedSubscriptions(t)
	cfg := subTestConfig()
	user := cfg.Users[0]
	for i, protocol := range []string{"ss2022", "trojan", "vlessenc"} {
		subscriptionCoreFixture(t, protocol, coreplan.Options{Protocol: protocol, Host: cfg.PublicAddress, SNI: "example.test", Port: 28444 + i, Cert: "cert", Key: "key"}, user)
	}
	cfg.Users[0].QuotaGB = 1
	out := &printOutput{}
	warnQuotaScope(cfg, out)
	if out.count("No counted protocol is installed") != 1 {
		t.Fatal("quota notice treated a counted-but-excluded core as a usable quota path")
	}
}
