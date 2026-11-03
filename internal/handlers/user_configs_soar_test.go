package handlers

import (
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
	if !strings.Contains(all, "[soar") || !strings.Contains(all, "slipnet://") {
		t.Fatalf("soar config missing:\n%s", all)
	}
}
