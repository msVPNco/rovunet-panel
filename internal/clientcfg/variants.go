package clientcfg

import (
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

// Variant is one client configuration a tunnel yields: the backend it uses
// and the tag (profile name) it carries.
type Variant struct{ Backend, Tag string }

// Variants returns the configurations a tunnel should print. A NaiveProxy
// tunnel is one Caddy forward proxy on :443 that serves both SOCKS and SSH
// clients (the client picks by its CONNECT target), so it yields two, "-socks"
// and "-ssh"; every other tunnel yields one with its own backend and tag.
func Variants(t *config.TunnelConfig) []Variant {
	if t.Transport != config.TransportNaive {
		return []Variant{{Backend: t.Backend, Tag: t.Tag}}
	}
	base := strings.TrimSuffix(t.Tag, "-socks")
	base = strings.TrimSuffix(base, "-ssh")
	return []Variant{
		{Backend: config.BackendSOCKS, Tag: base + "-socks"},
		{Backend: config.BackendSSH, Tag: base + "-ssh"},
	}
}

// PlainSOCKS reports whether a tunnel is the unencrypted direct SOCKS5 server.
// It is left out of the default ("essential") output: it is readable by
// anyone on the path, and was blocked on the filtered network we measured.
// Pass --all (export-clients) or set ROVUNET_ALL_CONFIGS=1 to list it.
func PlainSOCKS(t *config.TunnelConfig) bool {
	return t.Transport == config.TransportSOCKS
}

// HiddenNote is printed when PlainSOCKS tunnels were left out.
const HiddenNote = "# %d plain SOCKS5 config(s) hidden (unencrypted). Show them with: ROVUNET_ALL_CONFIGS=1 rovunet users, or export-clients --all"
