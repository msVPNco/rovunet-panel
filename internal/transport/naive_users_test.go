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
	c := buildCaddyfile(tun, users)
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
