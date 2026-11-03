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
func GenerateVLESSLink(tunnel *config.TunnelConfig, username, password string) (string, error) {
	v := tunnel.VLESS
	if v == nil {
		return "", fmt.Errorf("vless config is nil")
	}
	id, err := VLESSUserUUID(tunnel, username, password)
	if err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("type", "ws")
	q.Set("host", tunnel.Domain)
	q.Set("path", v.Path)
	if v.TLS {
		q.Set("security", "tls")
		q.Set("sni", tunnel.Domain)
		q.Set("fp", "chrome")
		q.Set("alpn", "http/1.1")
		if !v.CDN {
			// Straight to the origin, whose certificate is self-signed
			q.Set("allowInsecure", "1")
		}
	} else {
		q.Set("security", "none")
	}

	addr := v.ClientAddress(tunnel.Domain)
	if ip := net.ParseIP(addr); ip != nil && ip.To4() == nil {
		addr = "[" + addr + "]"
	}
	name := tunnel.Tag + "-" + username
	return "vless://" + id + "@" + addr + ":" + strconv.Itoa(v.Port) + "?" + q.Encode() + "#" + url.PathEscape(name), nil
}
