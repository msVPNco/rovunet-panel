package transport

import (
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
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

func TestCaddyfileHasNoDefaultLogin(t *testing.T) {
	tun := &config.TunnelConfig{Domain: "n.example.com", Naive: &config.NaiveConfig{Email: "a@b.c", Port: 443, DecoyURL: "https://www.example.org"}}
	c := buildCaddyfile(tun, nil, nil)
	// The upstream panel once shipped a well-known default login; neither
	// it nor a renamed equivalent may ever be accepted.
	if strings.Contains(c, "basic_auth slip"+"gate") || strings.Contains(c, "basic_auth rovunet") {
		t.Fatalf("well-known default login must not be accepted:\n%s", c)
	}
	if strings.Count(c, "basic_auth") != 1 || !strings.Contains(c, "basic_auth locked-") {
		t.Fatalf("a proxy without users must stay closed behind a random login:\n%s", c)
	}
	if again := buildCaddyfile(tun, nil, nil); again == c {
		t.Fatal("the placeholder login must be random")
	}
	c = buildCaddyfile(tun, []config.UserConfig{{Username: "alice", Password: "p1"}}, nil)
	if strings.Count(c, "basic_auth") != 1 || !strings.Contains(c, "basic_auth alice p1") {
		t.Fatalf("only the panel user should log in:\n%s", c)
	}
}

func TestCaddyfileDisablesHTTP3(t *testing.T) {
	tun := &config.TunnelConfig{Domain: "n.example.com", Naive: &config.NaiveConfig{Email: "a@b.c", Port: 443, User: "alice", Password: "p1", DecoyURL: "https://www.example.org"}}
	if c := buildCaddyfile(tun, nil, nil); !strings.Contains(c, "protocols h1 h2") {
		t.Fatalf("Caddy must not open UDP 443 for HTTP/3:\n%s", c)
	}
}

func TestCaddyfileServesSubscriptionsBeforeProxy(t *testing.T) {
	tun := &config.TunnelConfig{Tag: "naive", Domain: "n.example.com", Naive: &config.NaiveConfig{Email: "a@b.c", Port: 443, DecoyURL: "https://example.org"}}
	c := buildCaddyfile(tun, nil, nil)
	sub := strings.Index(c, "file_server @rovunet_sub")
	fp := strings.Index(c, "forward_proxy")
	if sub < 0 || fp < 0 || sub > fp {
		t.Fatalf("subscription files must be served before forward_proxy:\n%s", c)
	}
	if !strings.Contains(c, "path /sub/*") || strings.Contains(c, "browse") {
		t.Fatalf("only /sub/* is served and never listed:\n%s", c)
	}
}

func TestCaddyfileLeavesTrafficLimitedUsersOut(t *testing.T) {
	// The tunnel's own credential is carol's, from when she was the first user.
	tun := &config.TunnelConfig{Domain: "n.example.com", Naive: &config.NaiveConfig{
		Email: "a@b.c", Port: 443, User: "carol", Password: "pw-carol-xyz", DecoyURL: "https://www.example.org",
	}}
	users := []config.UserConfig{
		{Username: "alice", Password: "pa"},
		{Username: "carol", Password: "pw-carol-xyz", QuotaGB: 5},
	}
	for name, c := range map[string]string{
		"full":     buildCaddyfile(tun, users, nil),
		"fallback": buildCaddyfileSub(tun, users, nil, false),
	} {
		if !strings.Contains(c, "basic_auth alice pa") {
			t.Fatalf("%s: an unlimited user must stay:\n%s", name, c)
		}
		if strings.Contains(c, "carol") || strings.Contains(c, "pw-carol-xyz") {
			t.Fatalf("%s: a traffic-limited user must not be able to log in to a proxy that does not count traffic:\n%s", name, c)
		}
	}
}
