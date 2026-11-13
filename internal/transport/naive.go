package transport

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/warp"
)

// createNaiveService creates the Caddyfile and systemd service for NaiveProxy.
func createNaiveService(tunnel *config.TunnelConfig, cfg *config.Config, preserve bool) error {
	if tunnel.Naive == nil {
		return fmt.Errorf("naive config is nil")
	}

	backend := cfg.GetBackend(tunnel.Backend)
	if backend == nil {
		return fmt.Errorf("backend %q not found", tunnel.Backend)
	}

	tunnelDir := config.TunnelDir(tunnel.Tag)
	caddyfilePath := filepath.Join(tunnelDir, "Caddyfile")

	// Build Caddyfile. Keep an earlier random decoy so a rewrite (e.g. a
	// user change) doesn't flip the decoy site on every restart.
	if tunnel.Naive.DecoyURL == "" {
		if old, err := os.ReadFile(caddyfilePath); err == nil {
			tunnel.Naive.DecoyURL = existingDecoy(string(old))
		}
	}
	caddyfile := buildCaddyfile(tunnel, cfg.Users, vlessRoutes(cfg, tunnel.Tag))
	if cfg.SubscriptionV2 {
		caddyfile = withV2Route(caddyfile)
	} else {
		caddyfile = withDisabledV2Route(caddyfile)
	}
	if old, e := os.ReadFile(caddyfilePath); e == nil {
		caddyfile = preserveLockedAuth(string(old), caddyfile)
	}
	if err := validateAndWriteCaddy(filepath.Join(config.DefaultBinDir, "caddy-naive"), caddyfilePath, []byte(caddyfile)); err != nil {
		return err
	}

	binPath := filepath.Join(config.DefaultBinDir, "caddy-naive")

	svcUser := "root"
	svcGroup := "root"
	var env []string

	// When WARP is enabled, run under a dedicated user so outbound
	// forward-proxy traffic gets routed through the WireGuard interface.
	if cfg.Warp.Enabled {
		svcUser = warp.NaiveUser
		svcGroup = config.SystemGroup

		// Caddy stores ACME certs and runtime state under XDG_DATA_HOME.
		dataDir := filepath.Join(tunnelDir, ".caddy")
		_ = os.MkdirAll(dataDir, 0750)
		_ = exec.Command("chown", "-R", svcUser+":"+svcGroup, tunnelDir).Run()
		env = append(env, "XDG_DATA_HOME="+dataDir)
	}

	unit := &service.Unit{
		Name:         service.TunnelServiceName(tunnel.Tag),
		Description:  fmt.Sprintf("Rovunet Panel NaiveProxy: %s", tunnel.Tag),
		ExecStartPre: "+/usr/local/bin/rovunet subscriptions-before-serve",
		ExecStart:    fmt.Sprintf("%s run --config %s --adapter caddyfile", binPath, caddyfilePath),
		User:         svcUser,
		Group:        svcGroup,
		After:        "network.target",
		Restart:      "always",
		WorkingDir:   tunnelDir,
		Environment:  env,
	}

	return service.Apply(unit, preserve)
}

// vlessRoute is a WebSocket path Caddy hands to a local server: a VLESS
// server (plain HTTP) or the StunTLS proxy (TLS, self-signed).
type vlessRoute struct {
	Path string
	Port int
	TLS  bool
}

// vlessRoutes lists the VLESS tunnels fronted by the NaiveProxy tunnel tag.
func vlessRoutes(cfg *config.Config, naiveTag string) []vlessRoute {
	var r []vlessRoute
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if t.Transport == config.TransportVLESS && t.VLESS != nil && t.VLESS.Front == naiveTag {
			r = append(r, vlessRoute{Path: t.VLESS.Path, Port: t.VLESS.Port})
		}
		if s := t.StunTLS; t.Transport == config.TransportStunTLS && s != nil && s.Front == naiveTag && s.Path != "" {
			port := s.Port
			if port == 0 {
				port = 443
			}
			r = append(r, vlessRoute{Path: s.Path, Port: port, TLS: true})
		}
	}
	return r
}

func buildCaddyfile(tunnel *config.TunnelConfig, users []config.UserConfig, vless []vlessRoute) string {
	return buildCaddyfileSub(tunnel, users, vless, true)
}

// buildCaddyfileSub builds the Caddyfile, with or without the /sub/ file
// route (the route is left out only as a fallback if Caddy rejects it).
func buildCaddyfileSub(tunnel *config.TunnelConfig, users []config.UserConfig, vless []vlessRoute, withSub bool) string {
	naiveCfg := tunnel.Naive

	// No built-in default login: a well-known default credential (the upstream panel once had one)
	// would let anyone use the proxy. Without any credential the proxy gets a
	// random one, so it stays closed until a panel user exists.
	auth := naiveAuthLines(naiveCfg.User, naiveCfg.Password, users)
	if auth == "" {
		auth = naiveAuthLines("locked-"+randomToken(8), randomToken(24), nil)
	}

	decoy := naiveCfg.DecoyURL
	if decoy == "" {
		decoy = config.RandomDecoyURL()
	}

	port := naiveCfg.Port
	if port == 0 {
		port = 443
	}

	subLines := ""
	if withSub {
		subLines = subscriptionRouteLines()
	}

	return fmt.Sprintf(`{
  admin off
  log {
    output stdout
    level WARN
  }
  # HTTP/1.1 and HTTP/2 only: NaiveProxy clients use HTTP/2, and HTTP/3
  # would hold UDP 443, which Hysteria2 or other UDP services may need.
  servers {
    protocols h1 h2
  }
}

:%d, %s {
  tls %s
  route {
%s%s    forward_proxy {
%s      hide_ip
      hide_via
      probe_resistance
    }
    reverse_proxy %s {
      header_up Host {upstream_hostport}
    }
  }
}
`, port, tunnel.Domain, naiveCfg.Email, vlessRouteLines(vless), subLines, auth, decoy)
}

// SubscriptionDir is where the panel writes each user's subscription file.
// Files sit under sub/ and are named by an unguessable token. Variable for tests.
var SubscriptionDir = "/var/lib/rovunet/subscriptions"

// subscriptionRouteLines serves /sub/<token>.json and the status file
// /sub/<token>.status.json from SubscriptionDir as plain static files, before
// the proxy and the decoy. There is no listing and no application behind it:
// a path that is not a token's file is a 404. Nothing may keep a copy: the
// status changes every few minutes and the files name the user's token.
func subscriptionRouteLines() string {
	return "    @rovunet_sub path_regexp ^/sub/[0-9a-f]{32}(\\.windows)?(\\.status)?\\.json$\n" +
		"    header @rovunet_sub Cache-Control \"no-store\"\n" +
		"    root @rovunet_sub " + SubscriptionDir + "\n" +
		"    file_server @rovunet_sub\n"
}

// vlessRouteLines sends each fronted VLESS WebSocket path to its local
// server. They come before forward_proxy, and every other request still
// falls through to the decoy site, so the host looks like an ordinary
// website with a valid certificate on 443.
func vlessRouteLines(routes []vlessRoute) string {
	var b strings.Builder
	for _, r := range routes {
		if r.TLS {
			// StunTLS speaks TLS with its own self-signed certificate
			fmt.Fprintf(&b, "    reverse_proxy %s https://127.0.0.1:%d {\n      flush_interval -1\n      transport http {\n        tls_insecure_skip_verify\n      }\n    }\n", r.Path, r.Port)
			continue
		}
		fmt.Fprintf(&b, "    reverse_proxy %s 127.0.0.1:%d {\n      flush_interval -1\n    }\n", r.Path, r.Port)
	}
	return b.String()
}

// naiveAuthLines lists the tunnel's own credential (when set) plus one
// basic_auth line per panel user allowed on uncounted services, so each such
// user's client config (which carries that user's name and password)
// authenticates against the proxy.
func naiveAuthLines(user, pass string, users []config.UserConfig) string {
	var b strings.Builder
	seen := map[string]bool{}
	add := func(u, p string) {
		if u == "" || p == "" || seen[u] {
			return
		}
		seen[u] = true
		fmt.Fprintf(&b, "      basic_auth %s %s\n", caddyQuote(u), caddyQuote(p))
	}
	// The tunnel's own credential is the first panel user's name and password;
	// it is dropped when that user has since been given a traffic limit.
	tunnelCredentialLimited := false
	for _, u := range users {
		if u.Username == user && u.MeteredOnly() {
			tunnelCredentialLimited = true
		}
	}
	if !tunnelCredentialLimited {
		add(user, pass)
	}
	// A traffic-limited user is left out: this proxy does not count traffic.
	for _, u := range config.UnmeteredUsers(users) {
		add(u.Username, u.Password)
	}
	return b.String()
}

// caddyQuote returns a Caddyfile token that survives spaces, quotes and
// backslashes in passwords.
func caddyQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"\\{}#`\n") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}

// existingDecoy pulls the reverse_proxy target out of a Caddyfile we wrote.
func existingDecoy(caddyfile string) string {
	for _, line := range strings.Split(caddyfile, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "reverse_proxy" && strings.HasPrefix(f[1], "http") {
			return f[1]
		}
	}
	return ""
}

// SyncNaive rewrites every NaiveProxy Caddyfile with the current users and
// fronted VLESS paths, and restarts the service when it changed.
func SyncNaive(cfg *config.Config) error {
	var firstErr error
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if t.Transport != config.TransportNaive || t.Naive == nil {
			continue
		}
		path := filepath.Join(config.TunnelDir(t.Tag), "Caddyfile")
		old, err := os.ReadFile(path)
		if err != nil {
			continue // not installed on this host
		}
		if t.Naive.DecoyURL == "" {
			t.Naive.DecoyURL = existingDecoy(string(old))
		}
		next := buildCaddyfile(t, cfg.Users, vlessRoutes(cfg, t.Tag))
		if cfg.SubscriptionV2 {
			next = withV2Route(next)
		} else {
			next = withDisabledV2Route(next)
		}
		next = preserveLockedAuth(string(old), next)
		if next == string(old) {
			continue
		}
		// Validate a candidate first: a bad Caddyfile would take NaiveProxy
		// (and everything it fronts) down on restart.
		candidate := path + ".new"
		if err := os.WriteFile(candidate, []byte(next), 0600); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		bin := filepath.Join(config.DefaultBinDir, "caddy-naive")
		if outB, err := exec.Command(bin, "validate", "--config", candidate, "--adapter", "caddyfile").CombinedOutput(); err != nil {
			_ = os.Remove(candidate)
			if firstErr == nil {
				firstErr = fmt.Errorf("new Caddyfile rejected, kept the old one: %v: %s", err, lastLine(string(outB)))
			}
			continue
		}
		// Keep the old file's owner: with WARP, Caddy runs as its own user
		if fi, err := os.Stat(path); err == nil {
			if st, ok := fi.Sys().(*syscall.Stat_t); ok {
				_ = os.Chown(candidate, int(st.Uid), int(st.Gid))
			}
		}
		if err := os.Rename(candidate, path); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := service.Restart(service.TunnelServiceName(t.Tag)); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// randomToken returns n random bytes as hex.
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// A rejected candidate must not remove the existing subscription route or
// replace its healthy Caddyfile. Engine diagnostics may contain private data.
func validateAndWriteCaddy(bin, path string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".Caddyfile-validate-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := exec.Command(bin, "validate", "--config", tmp.Name(), "--adapter", "caddyfile").Run(); err != nil {
		return fmt.Errorf("Caddy rejected the private configuration; existing Caddyfile and subscription route were retained")
	}
	return safefile.Write(path, body, 0600)
}

func withV2Route(body string) string {
	route := "    @rovunet_v2 path /v2/sub/*\n    header @rovunet_v2 Cache-Control \"no-store\"\n    reverse_proxy @rovunet_v2 unix/" + config.SubscriptionV2Socket + "\n"
	return strings.Replace(body, "  route {\n", "  route {\n"+route, 1)
}

var lockedAuth = regexp.MustCompile("(?m)^ *basic_auth locked-[0-9a-f]+ [0-9a-f]+$")

func preserveLockedAuth(old, next string) string {
	prior := lockedAuth.FindString(old)
	if prior == "" || lockedAuth.FindString(next) == "" {
		return next
	}
	return lockedAuth.ReplaceAllString(next, prior)
}

func withDisabledV2Route(body string) string {
	route := "    @rovunet_v2 path /v2/sub/*\n    header @rovunet_v2 Cache-Control \"no-store\"\n    respond @rovunet_v2 404\n"
	return strings.Replace(body, "  route {\n", "  route {\n"+route, 1)
}
