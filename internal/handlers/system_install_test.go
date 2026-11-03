package handlers

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/keys"
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

// TestPrintClientConfigs: tunnels with a login get one config per user,
// direct Soar gets one for everyone.
func TestPrintClientConfigs(t *testing.T) {
	cfg := &config.Config{
		Backends: config.DefaultBackends(),
		Users:    []config.UserConfig{{Username: "alice", Password: "a"}, {Username: "bob", Password: "b"}},
	}
	out := &printOutput{}
	// soar-ssh and dnstt + noizdns for each of two users, plus direct Soar
	if n := printClientConfigs(cfg, out, configsTestTunnels()); n != 7 {
		t.Errorf("printed %d configs, want 7", n)
	}
	if n := out.count("slipnet://"); n != 7 {
		t.Errorf("%d slipnet:// lines, want 7:\n%s", n, strings.Join(out.lines, "\n"))
	}
	for _, label := range []string{"[soar] (no login)", "[soar-ssh] alice", "[soar-ssh] bob", "[dnstt] bob", "[noizdns] alice"} {
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
// gets a slipnet:// and a vless:// config; without users there is none.
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
	if out.count("slipnet://") != 2 || out.count("vless://") != 2 || out.count("[vless] alice") != 1 || out.count("[vless] bob") != 1 {
		t.Errorf("got:\n%s", strings.Join(out.lines, "\n"))
	}

	cfg.Users = nil
	out = &printOutput{}
	if n := printClientConfigs(cfg, out, tunnels); n != 0 {
		t.Errorf("printed %d configs without users, want 0", n)
	}
}
