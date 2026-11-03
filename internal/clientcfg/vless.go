package clientcfg

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/keys"
)

// VLESSUserUUID returns the VLESS UUID of a SlipGate user on a VLESS tunnel.
// It is derived from the user's credentials and the tunnel's secret, the same
// way the server checks it, so it changes when the password does.
func VLESSUserUUID(tunnel *config.TunnelConfig, username, password string) (string, error) {
	if tunnel.VLESS == nil {
		return "", fmt.Errorf("vless config is nil")
	}
	if username == "" {
		return "", fmt.Errorf("vless configs are per user: choose a user (create one with: slipgate users add)")
	}
	secret, err := keys.LoadVLESSSecret(tunnel.VLESS.SecretFile)
	if err != nil {
		return "", err
	}
	return keys.FormatUUID(keys.VLESSUserID(secret, username, password)), nil
}

// GenerateVLESSLink builds a standard vless:// link for a user, for clients
// such as v2rayNG, Hiddify, NekoBox or Streisand (SlipNet imports it too).
func GenerateVLESSLink(tunnel *config.TunnelConfig, cfg *config.Config, username, password string) (string, error) {
	v := tunnel.VLESS
	if v == nil {
		return "", fmt.Errorf("vless config is nil")
	}
	id, err := VLESSUserUUID(tunnel, username, password)
	if err != nil {
		return "", err
	}

	ep := VLESSEndpointFor(tunnel, cfg)
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("type", "ws")
	q.Set("host", ep.Host)
	q.Set("path", v.Path)
	if ep.TLS {
		q.Set("security", "tls")
		q.Set("sni", ep.Host)
		q.Set("fp", "chrome")
		q.Set("alpn", "http/1.1")
		if ep.SelfSigned {
			// Straight to the origin, whose certificate is self-signed.
			// Xray clients drop this flag from 2026-08; front the tunnel
			// with NaiveProxy's Caddy for a real certificate instead.
			q.Set("allowInsecure", "1")
		}
	} else {
		q.Set("security", "none")
	}

	addr := ep.Address
	if ip := net.ParseIP(addr); ip != nil && ip.To4() == nil {
		addr = "[" + addr + "]"
	}
	name := tunnel.Tag + "-" + username
	return "vless://" + id + "@" + addr + ":" + strconv.Itoa(ep.Port) + "?" + q.Encode() + "#" + url.PathEscape(name), nil
}

// VLESSEndpoint is where and how a client reaches a VLESS tunnel.
type VLESSEndpoint struct {
	Host       string // WebSocket Host and TLS SNI
	Address    string // address the client dials
	Port       int
	TLS        bool
	SelfSigned bool // TLS straight to the origin's self-signed certificate
	Fronted    bool // served by NaiveProxy's Caddy with a real certificate
}

// VLESSEndpointFor resolves the client-facing endpoint: through the fronting
// NaiveProxy (its domain, port and Let's Encrypt certificate), through the
// CDN, or straight to the origin.
func VLESSEndpointFor(tunnel *config.TunnelConfig, cfg *config.Config) VLESSEndpoint {
	v := tunnel.VLESS
	if cfg != nil {
		if f := cfg.VLESSFront(tunnel); f != nil {
			port := f.Naive.Port
			if port == 0 {
				port = 443
			}
			return VLESSEndpoint{Host: f.Domain, Address: f.Domain, Port: port, TLS: true, Fronted: true}
		}
	}
	return VLESSEndpoint{
		Host:       tunnel.Domain,
		Address:    v.ClientAddress(tunnel.Domain),
		Port:       v.Port,
		TLS:        v.TLS,
		SelfSigned: v.TLS && !v.CDN,
	}
}
