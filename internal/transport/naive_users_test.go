package transport

import (
	"strings"
	"testing"

	"github.com/anonvector/slipgate/internal/config"
)

func TestCaddyfileHasEveryUser(t *testing.T) {
	tun := &config.TunnelConfig{Domain: "n.example.com", Naive: &config.NaiveConfig{
		Email: "a@b.c", Port: 443, User: "alice", Password: "p1", DecoyURL: "https://www.example.org",
	}}
	users := []config.UserConfig{{Username: "alice", Password: "p1"}, {Username: "bob", Password: `we ird"\pw`}}
	c := buildCaddyfile(tun, users, []vlessRoute{{Path: "/api/x", Port: 2053}})
	if strings.Count(c, "basic_auth") != 2 {
		t.Fatalf("want 2 basic_auth lines:\n%s", c)
	}
	if !strings.Contains(c, `basic_auth bob "we ird\"\\pw"`) {
		t.Fatalf("quoting wrong:\n%s", c)
	}
	if existingDecoy(c) != "https://www.example.org" {
		t.Fatalf("decoy not recovered")
	}
}

func TestCaddyfileFrontsVLESSBeforeProxy(t *testing.T) {
	tun := &config.TunnelConfig{Domain: "n.example.com", Naive: &config.NaiveConfig{Email: "a@b.c", Port: 443, DecoyURL: "https://www.example.org"}}
	c := buildCaddyfile(tun, nil, []vlessRoute{{Path: "/api/x", Port: 2053}})
	rp := strings.Index(c, "reverse_proxy /api/x 127.0.0.1:2053")
	fp := strings.Index(c, "forward_proxy")
	if rp < 0 || fp < 0 || rp > fp {
		t.Fatalf("vless route must come before forward_proxy:\n%s", c)
	}
}

func TestCaddyfileFrontsStunTLSOverTLS(t *testing.T) {
	cfg := &config.Config{Tunnels: []config.TunnelConfig{
		{Tag: "naive", Transport: config.TransportNaive, Domain: "n.example.com", Naive: &config.NaiveConfig{Email: "a@b.c", Port: 443, DecoyURL: "https://www.example.org"}},
		{Tag: "stuntls", Transport: config.TransportStunTLS, StunTLS: &config.StunTLSConfig{Port: 8443, Front: "naive", Path: "/ws/abc"}},
	}}
	c := buildCaddyfile(&cfg.Tunnels[0], nil, vlessRoutes(cfg, "naive"))
	for _, want := range []string{"reverse_proxy /ws/abc https://127.0.0.1:8443 {", "tls_insecure_skip_verify"} {
		if !strings.Contains(c, want) {
			t.Fatalf("missing %q:\n%s", want, c)
		}
	}
	if strings.Index(c, "/ws/abc") > strings.Index(c, "forward_proxy") {
		t.Fatal("stuntls route must precede forward_proxy")
	}
}
