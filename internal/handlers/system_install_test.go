package handlers

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/keys"
)

// printOutput records what's printed.
type printOutput struct{ lines []string }

func (o *printOutput) Info(msg string)    { o.lines = append(o.lines, msg) }
func (o *printOutput) Success(msg string) { o.lines = append(o.lines, msg) }
func (o *printOutput) Warning(msg string) { o.lines = append(o.lines, msg) }
func (o *printOutput) Error(msg string)   { o.lines = append(o.lines, msg) }
func (o *printOutput) Print(msg string)   { o.lines = append(o.lines, msg) }

func (o *printOutput) count(substr string) int {
	n := 0
	for _, l := range o.lines {
		if strings.Contains(l, substr) {
			n++
		}
	}
	return n
}

func configsTestTunnels() []config.TunnelConfig {
	soarKey := &config.SoarConfig{PublicKey: "tCd7Qm7FolQBkQfQUWLVrLBktDMirri+SlUaAHT1gxM="}
	return []config.TunnelConfig{
		{Tag: "soar", Transport: config.TransportSoar, Backend: config.BackendDirect, Domain: "r.example.com", Soar: soarKey},
		{Tag: "soar-ssh", Transport: config.TransportSoar, Backend: config.BackendSSH, Domain: "rs.example.com", Soar: soarKey},
		{Tag: "dnstt", Transport: config.TransportDNSTT, Backend: config.BackendSOCKS, Domain: "t.example.com",
			DNSTT: &config.DNSTTConfig{PublicKey: strings.Repeat("ab", 32)}},
	}
}

// TestPrintClientConfigs: every tunnel gets one config per user. Direct Soar
// needs no login but is printed per user too, so the app puts it in that
// user's group instead of a group of its own.
func TestPrintClientConfigs(t *testing.T) {
	cfg := &config.Config{
		Backends: config.DefaultBackends(),
		Users:    []config.UserConfig{{Username: "alice", Password: "a"}, {Username: "bob", Password: "b"}},
	}
	out := &printOutput{}
	// direct Soar, soar-ssh and dnstt + noizdns for each of two users
	if n := printClientConfigs(cfg, out, configsTestTunnels()); n != 8 {
		t.Errorf("printed %d configs, want 8", n)
	}
	if n := out.count("rovunet://"); n != 8 {
		t.Errorf("%d rovunet:// lines, want 8:\n%s", n, strings.Join(out.lines, "\n"))
	}
	for _, label := range []string{"[soar] alice, no login needed", "[soar] bob, no login needed", "[soar-ssh] alice", "[soar-ssh] bob", "[dnstt] bob", "[noizdns] alice"} {
		if out.count(label) != 1 {
			t.Errorf("want one %q:\n%s", label, strings.Join(out.lines, "\n"))
		}
	}
}

// TestPrintClientConfigsNoUsers: without users, direct Soar still has its
// config and the others get one without credentials.
func TestPrintClientConfigsNoUsers(t *testing.T) {
	cfg := &config.Config{Backends: config.DefaultBackends()}
	out := &printOutput{}
	if n := printClientConfigs(cfg, out, configsTestTunnels()); n != 4 {
		t.Errorf("printed %d configs, want 4:\n%s", n, strings.Join(out.lines, "\n"))
	}
	if out.count("[soar] (no login)") != 1 || out.count("(no auth)") != 3 {
		t.Errorf("got:\n%s", strings.Join(out.lines, "\n"))
	}
}

// TestPrintClientConfigsVLESS: VLESS has no backend config, yet every user
// gets a rovunet:// and a vless:// config; without users there is none.
func TestPrintClientConfigsVLESS(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "vless.secret")
	if err := keys.GenerateVLESSSecret(secret); err != nil {
		t.Fatal(err)
	}
	tunnels := []config.TunnelConfig{{
		Tag: "vless", Transport: config.TransportVLESS, Backend: config.BackendDirect, Domain: "cdn.example.com",
		VLESS: &config.VLESSConfig{Port: 2053, Path: "/p", TLS: true, Cert: "c", Key: "k", SecretFile: secret, CDN: true},
	}}
	cfg := &config.Config{
		Backends: config.DefaultBackends(),
		Users:    []config.UserConfig{{Username: "alice", Password: "a"}, {Username: "bob", Password: "b"}},
	}
	out := &printOutput{}
	if n := printClientConfigs(cfg, out, tunnels); n != 2 {
		t.Errorf("printed %d configs, want 2", n)
	}
	if out.count("rovunet://") != 2 || out.count("vless://") != 2 || out.count("[vless] alice") != 1 || out.count("[vless] bob") != 1 {
		t.Errorf("got:\n%s", strings.Join(out.lines, "\n"))
	}

	cfg.Users = nil
	out = &printOutput{}
	if n := printClientConfigs(cfg, out, tunnels); n != 0 {
		t.Errorf("printed %d configs without users, want 0", n)
	}
}

// TestShowUserConfigsNaiveBothModes: one NaiveProxy tunnel serves SOCKS and
// SSH clients, so `users configs` must print both, like the install summary.
func TestShowUserConfigsNaiveBothModes(t *testing.T) {
	cfg := &config.Config{
		PublicAddress: "203.0.113.7",
		Backends:      config.DefaultBackends(),
		Tunnels: []config.TunnelConfig{{
			Tag: "naive", Transport: config.TransportNaive, Backend: config.BackendSOCKS,
			Domain: "n.example.com", Naive: &config.NaiveConfig{Port: 443},
		}},
	}
	out := &printOutput{}
	showUserConfigs(cfg, "alice", "pw", out)
	all := strings.Join(out.lines, "\n")
	if out.count("[naive-socks] user=alice") != 1 || out.count("[naive-ssh] user=alice") != 1 || out.count("rovunet://") != 2 {
		t.Fatalf("want naive-socks and naive-ssh:\n%s", all)
	}
}

// TestPlainSOCKSHiddenByDefault: the unencrypted direct SOCKS5 tunnel is left
// out of the default output, with a note, and listed with ROVUNET_ALL_CONFIGS=1.
func TestPlainSOCKSHiddenByDefault(t *testing.T) {
	cfg := &config.Config{
		PublicAddress: "203.0.113.7",
		Backends:      config.DefaultBackends(),
		Users:         []config.UserConfig{{Username: "alice", Password: "a"}},
		Tunnels: []config.TunnelConfig{
			{Tag: "socks", Transport: config.TransportSOCKS, Backend: config.BackendSOCKS, Enabled: true},
		},
	}
	t.Setenv("ROVUNET_ALL_CONFIGS", "")
	out := &printOutput{}
	showUserConfigs(cfg, "alice", "a", out)
	if out.count("[socks]") != 0 || out.count("plain SOCKS5 config(s) hidden") != 1 {
		t.Fatalf("default output:\n%s", strings.Join(out.lines, "\n"))
	}
	t.Setenv("ROVUNET_ALL_CONFIGS", "1")
	out = &printOutput{}
	showUserConfigs(cfg, "alice", "a", out)
	if out.count("[socks]") != 1 || out.count("hidden") != 0 {
		t.Fatalf("with ROVUNET_ALL_CONFIGS=1:\n%s", strings.Join(out.lines, "\n"))
	}
}
