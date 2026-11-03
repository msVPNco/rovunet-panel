package transport

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/service"
	"github.com/anonvector/slipgate/internal/warp"
)

// createNaiveService creates the Caddyfile and systemd service for NaiveProxy.
func createNaiveService(tunnel *config.TunnelConfig, cfg *config.Config) error {
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
	caddyfile := buildCaddyfile(tunnel, cfg.Users)
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0600); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
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
		Name:        service.TunnelServiceName(tunnel.Tag),
		Description: fmt.Sprintf("SlipGate NaiveProxy: %s", tunnel.Tag),
		ExecStart:   fmt.Sprintf("%s run --config %s --adapter caddyfile", binPath, caddyfilePath),
		User:        svcUser,
		Group:       svcGroup,
		After:       "network.target",
		Restart:     "always",
		WorkingDir:  tunnelDir,
		Environment: env,
	}

	if err := service.Create(unit); err != nil {
		return err
	}

	// Restart if already running (e.g. Caddyfile updated with new credentials)
	if _, err := service.Status(unit.Name); err == nil {
		return service.Restart(unit.Name)
	}
	return service.Start(unit.Name)
}

func buildCaddyfile(tunnel *config.TunnelConfig, users []config.UserConfig) string {
	naiveCfg := tunnel.Naive

	user := naiveCfg.User
	pass := naiveCfg.Password
	if user == "" {
		user = "slipgate"
	}
	if pass == "" {
		pass = "slipgate"
	}

	decoy := naiveCfg.DecoyURL
	if decoy == "" {
		decoy = config.RandomDecoyURL()
	}

	port := naiveCfg.Port
	if port == 0 {
		port = 443
	}

	return fmt.Sprintf(`{
  admin off
  log {
    output stdout
    level WARN
  }
}

:%d, %s {
  tls %s
  route {
    forward_proxy {
%s      hide_ip
      hide_via
      probe_resistance
    }
    reverse_proxy %s {
      header_up Host {upstream_hostport}
    }
  }
}
`, port, tunnel.Domain, naiveCfg.Email, naiveAuthLines(user, pass, users), decoy)
}

// naiveAuthLines lists the tunnel's own credential plus one basic_auth line
// per SlipGate user, so every user's client config (which carries that
// user's name and password) authenticates against the proxy.
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
	add(user, pass)
	for _, u := range users {
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

// SyncNaiveUsers rewrites every NaiveProxy Caddyfile with the current user
// list and restarts the service when it changed.
func SyncNaiveUsers(cfg *config.Config) error {
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
		next := buildCaddyfile(t, cfg.Users)
		if next == string(old) {
			continue
		}
		if err := os.WriteFile(path, []byte(next), 0600); err != nil {
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
