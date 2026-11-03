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
