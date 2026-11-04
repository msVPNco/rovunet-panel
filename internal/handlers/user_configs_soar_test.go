package handlers

import (
	"github.com/anonvector/slipgate/internal/keys"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anonvector/slipgate/internal/config"
)

func TestUserConfigsIncludeSoarDirect(t *testing.T) {
	cfg := &config.Config{
		PublicAddress: "203.0.113.7",
		Tunnels: []config.TunnelConfig{{
			Tag: "soar", Transport: config.TransportSoar, Backend: config.BackendDirect,
			Domain: "r.example.com", Port: 5316,
			Soar: &config.SoarConfig{PublicKey: strings.Repeat("ab", 32)},
		}},
	}
	out := &printOutput{}
	showUserConfigs(cfg, "alice", "pw", out)
	all := strings.Join(out.lines, "\n")
	if !strings.Contains(all, "[soar") || !strings.Contains(all, "rovunet://") {
		t.Fatalf("soar config missing:\n%s", all)
	}
}

func TestFullPanelTextContract(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "vless.secret")
	if err := keys.GenerateVLESSSecret(secret); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends(), Tunnels: []config.TunnelConfig{
		{Tag: "soar", Transport: config.TransportSoar, Backend: config.BackendDirect, Domain: "r.example.org", Soar: &config.SoarConfig{PublicKey: strings.Repeat("ab", 32)}},
		{Tag: "vless", Transport: config.TransportVLESS, Backend: config.BackendDirect, Domain: "example.org", VLESS: &config.VLESSConfig{Port: 443, Path: "/test", TLS: true, CDN: true, Cert: "test-cert", Key: "test-key", SecretFile: secret}},
	}}
	out := &printOutput{}
	for _, user := range []string{"alice", "bob"} {
		showUserConfigs(cfg, user, "fixture-only", out)
	}
	body := strings.Join(out.lines, "\n")
	for _, user := range []string{"alice", "bob"} {
		if !strings.Contains(body, "[vless] user="+user) {
			t.Fatal("missing explicit account heading")
		}
	}
	if strings.Count(body, "rovunet://") != 4 || strings.Count(body, "  vless://") != 2 {
		t.Fatal("incomplete real panel output")
	}
	if path := os.Getenv("ROVUNET_PANEL_TEXT_FIXTURE"); path != "" {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
