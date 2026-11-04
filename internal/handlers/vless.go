package handlers

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anonvector/slipgate/internal/actions"
	"github.com/anonvector/slipgate/internal/certs"
	"github.com/anonvector/slipgate/internal/clientcfg"
	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/keys"
	"github.com/anonvector/slipgate/internal/prompt"
)

// setupVLESS asks for (or takes from flags) a VLESS tunnel's settings and
// creates its certificate in tunnelDir. With quiet it asks nothing and takes
// the usual setup behind Cloudflare: proxied domain, TLS to the origin, a free
// Cloudflare port, and the path derived from the server secret.
func setupVLESS(ctx *actions.Context, cfg *config.Config, tunnelDir, domain string, quiet bool) (*config.VLESSConfig, error) {
	out := ctx.Output
	v := &config.VLESSConfig{}

	// One secret for the whole server, kept when a tunnel is removed, so the
	// same domain always gets the same path and users keep their UUIDs.
	if err := keys.EnsureVLESSSecret(config.DefaultVLESSSecret); err != nil {
		return nil, err
	}
	secret, err := keys.LoadVLESSSecret(config.DefaultVLESSSecret)
	if err != nil {
		return nil, err
	}

	// The domain goes into every client config as Host and SNI
	if net.ParseIP(domain) != nil || config.ValidatePublicAddress(domain) != nil {
		return nil, fmt.Errorf("invalid domain %q: give the hostname clients connect to, e.g. cdn.example.com", domain)
	}

	if v.CDN, err = argOrConfirm(ctx, "cdn", "Is the domain proxied by Cloudflare (orange cloud)?", quiet); err != nil {
		return nil, err
	}

	// TLS to the origin keeps the hop between the CDN and this server
	// private. Without a CDN it is mandatory: plain HTTP would show every
	// destination to DPI.
	v.TLS = true
	if v.CDN {
		if v.TLS, err = argOrConfirm(ctx, "tls", "Use TLS between Cloudflare and this server? (Cloudflare SSL/TLS mode: Full)", quiet); err != nil {
			return nil, err
		}
	}

	portStr := ctx.GetArg("listen-port")
	if portStr == "" && quiet {
		portStr = strconv.Itoa(suggestVLESSPort(cfg, v.TLS))
	}
	if portStr == "" {
		portStr, err = prompt.String("Listen port", strconv.Itoa(suggestVLESSPort(cfg, v.TLS)))
		if err != nil {
			return nil, err
		}
	}
	if v.Port, err = strconv.Atoi(strings.TrimSpace(portStr)); err != nil {
		return nil, fmt.Errorf("invalid port %q", portStr)
	}
	if other := tcpPortOwner(cfg, v.Port, ""); other != "" {
		return nil, fmt.Errorf("port %d is already used by tunnel %q", v.Port, other)
	}
	if v.CDN && !v.CloudflarePortOK() {
		ports := config.CloudflareHTTPSPorts
		if !v.TLS {
			ports = config.CloudflareHTTPPorts
		}
		out.Warning(fmt.Sprintf("Cloudflare doesn't proxy port %d; use one of %s", v.Port, joinPorts(ports)))
	}

	v.Path = ctx.GetArg("path")
	if v.Path == "" {
		def := keys.VLESSDefaultPath(secret, domain)
		if quiet {
			v.Path = def
		} else if v.Path, err = prompt.String("WebSocket path", def); err != nil {
			return nil, err
		}
	}
	if !strings.HasPrefix(v.Path, "/") {
		v.Path = "/" + v.Path
	}

	v.CDNAddress = ctx.GetArg("cdn-address")
	if v.CDNAddress == "" && v.CDN && !quiet {
		if v.CDNAddress, err = prompt.String("Clean Cloudflare IP for client configs (blank = use the domain)", ""); err != nil {
			return nil, err
		}
	}
	v.CDNAddress = strings.TrimSpace(v.CDNAddress)

	v.BlockPorts = ctx.GetArg("block-ports")

	v.SecretFile = config.DefaultVLESSSecret
	if v.TLS {
		v.Cert = filepath.Join(tunnelDir, "cert.pem")
		v.Key = filepath.Join(tunnelDir, "key.pem")
		out.Info("Generating self-signed TLS certificate...")
		if err := certs.GenerateSelfSigned(v.Cert, v.Key, domain); err != nil {
			return nil, fmt.Errorf("cert generation failed: %w", err)
		}
	}

	if err := config.ValidateVLESS(v); err != nil {
		return nil, err
	}
	return v, nil
}

// argOrConfirm reads a yes/no flag ("yes"/"no", "true"/"false") or asks,
// defaulting to yes (without asking when quiet).
func argOrConfirm(ctx *actions.Context, key, question string, quiet bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(ctx.GetArg(key))) {
	case "yes", "y", "true", "1", "on":
		return true, nil
	case "no", "n", "false", "0", "off":
		return false, nil
	case "":
		if quiet {
			return true, nil
		}
		return prompt.ConfirmYes(question)
	default:
		return false, fmt.Errorf("--%s takes yes or no", key)
	}
}

// suggestVLESSPort picks the first Cloudflare-proxied port that no tunnel
// uses and nothing on the system listens on.
func suggestVLESSPort(cfg *config.Config, useTLS bool) int {
	ports := config.CloudflareHTTPSPorts
	if !useTLS {
		ports = config.CloudflareHTTPPorts
	}
	for _, p := range ports {
		if tcpPortOwner(cfg, p, "") == "" && tcpPortFree(p) {
			return p
		}
	}
	return ports[len(ports)-1]
}

// tcpPortOwner returns the tag of a tunnel (other than skipTag) listening on
// TCP port p, or "".
func tcpPortOwner(cfg *config.Config, p int, skipTag string) string {
	for _, t := range cfg.Tunnels {
		if t.Tag == skipTag {
			continue
		}
		var port int
		switch {
		case t.Naive != nil:
			port = t.Naive.Port
			if port == 0 {
				port = 443
			}
		case t.StunTLS != nil:
			port = t.StunTLS.Port
		case t.VLESS != nil:
			port = t.VLESS.Port
		}
		if port == p {
			return t.Tag
		}
	}
	return ""
}

func tcpPortFree(p int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(p))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func joinPorts(ports []int) string {
	s := make([]string, len(ports))
	for i, p := range ports {
		s[i] = strconv.Itoa(p)
	}
	return strings.Join(s, ", ")
}

// printVLESSSetup tells the operator what to set at Cloudflare.
func printVLESSSetup(out actions.OutputWriter, cfg *config.Config, t *config.TunnelConfig) {
	v := t.VLESS
	if v == nil {
		return
	}
	out.Print("")
	if f := cfg.VLESSFront(t); f != nil {
		out.Info(fmt.Sprintf("VLESS is served by NaiveProxy on %s:%d with its Let's Encrypt certificate", f.Domain, f.Naive.Port))
		out.Print("    DNS        : " + f.Domain + " stays DNS only (grey cloud); " + t.Domain + " is no longer needed")
		out.Print("    Wrong paths and probes see the decoy website, like any other HTTPS site")
		return
	}
	if v.CDN {
		out.Info("Cloudflare setup for " + t.Domain + ":")
		out.Print("    DNS        : A record " + t.Domain + " → this server's IP, Proxy status: Proxied (orange cloud)")
		if v.TLS {
			out.Print("    SSL/TLS    : encryption mode Full (Flexible breaks it; Full (strict) rejects the self-signed origin cert)")
		} else {
			out.Print("    SSL/TLS    : clients connect over plain HTTP on port " + strconv.Itoa(v.Port) + "; this is weaker than TLS")
		}
		out.Print("    Network    : WebSockets On (the default)")
		out.Print("    Other DNS records (DNS tunnels, NaiveProxy) stay DNS only (grey cloud)")
		if !v.CloudflarePortOK() {
			out.Warning(fmt.Sprintf("Port %d is not one Cloudflare proxies; clients won't connect", v.Port))
		}
	} else {
		out.Info("Point " + t.Domain + " at this server (A record; at Cloudflare: DNS only, grey cloud)")
	}
	out.Print("    In the Rovunet app you can replace the CDN address with a clean Cloudflare IP at any time")
}

// handleVLESSShare prints a user's SlipNet and standard vless:// links.
func handleVLESSShare(ctx *actions.Context, cfg *config.Config, tunnel *config.TunnelConfig) error {
	if len(cfg.Users) == 0 {
		return actions.NewErrorWithHint(actions.TunnelShare, "VLESS configs are per user and there are no users",
			"Create one with: sudo rovunet users add", nil)
	}
	username := ctx.GetArg("user")
	if username == "" {
		opts := make([]actions.SelectOption, 0, len(cfg.Users))
		for _, u := range cfg.Users {
			opts = append(opts, actions.SelectOption{Value: u.Username, Label: u.Username})
		}
		var err error
		if username, err = prompt.Select("User", opts); err != nil {
			return err
		}
	}
	user := cfg.GetUser(username)
	if user == nil {
		return actions.NewError(actions.TunnelShare, fmt.Sprintf("user %q not found", username), nil)
	}
	printVLESSLinks(ctx.Output, cfg, tunnel, user.Username, user.Password)
	return nil
}

func printVLESSLinks(out actions.OutputWriter, cfg *config.Config, tunnel *config.TunnelConfig, username, password string) {
	uri, err := clientcfg.GeneratePortableURI(tunnel, nil, cfg, clientcfg.URIOptions{Username: username, Password: password}, "rovunet")
	if err != nil {
		out.Warning(fmt.Sprintf("[%s] %v", tunnel.Tag, err))
		return
	}
	link, err := clientcfg.GenerateVLESSLink(tunnel, cfg, username, password)
	if err != nil {
		out.Warning(fmt.Sprintf("[%s] %v", tunnel.Tag, err))
		return
	}
	out.Print(fmt.Sprintf("  [%s] user=%s", tunnel.Tag, username))
	out.Print("  " + uri)
	out.Print("")
	out.Print(fmt.Sprintf("  [%s] user=%s", tunnel.Tag, username))
	out.Print("  " + link)
	out.Print("")
}
