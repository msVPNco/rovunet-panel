package config

import "strings"

// Transport types.
const (
	TransportDNSTT      = "dnstt"
	TransportSlipstream = "slipstream"
	TransportVayDNS     = "vaydns"
	TransportSoar       = "soar"
	TransportNaive      = "naive"
	TransportStunTLS    = "stuntls"
	TransportVLESS      = "vless"
	TransportSSH        = "direct-ssh"
	TransportSOCKS      = "direct-socks5"
	TransportExternal   = "external"
)

// TunnelConfig defines a single tunnel.
type TunnelConfig struct {
	Tag       string `json:"tag"`
	Transport string `json:"transport"`
	Backend   string `json:"backend"`
	Domain    string `json:"domain"`
	Port      int    `json:"port,omitempty"` // DNS tunnels: internal forwarding port (5310+)
	Enabled   bool   `json:"enabled"`

	// Transport-specific configs (only one set per tunnel)
	DNSTT      *DNSTTConfig      `json:"dnstt,omitempty"`
	Slipstream *SlipstreamConfig `json:"slipstream,omitempty"`
	VayDNS     *VayDNSConfig     `json:"vaydns,omitempty"`
	Soar       *SoarConfig       `json:"soar,omitempty"`
	Naive      *NaiveConfig      `json:"naive,omitempty"`
	StunTLS    *StunTLSConfig    `json:"stuntls,omitempty"`
	VLESS      *VLESSConfig      `json:"vless,omitempty"`
}

// DNSTTConfig holds config for DNSTT transport (serves both DNSTT and NoizDNS clients).
type DNSTTConfig struct {
	MTU        int    `json:"mtu"`
	PrivateKey string `json:"private_key"` // path to key file
	PublicKey  string `json:"public_key"`  // hex-encoded public key
}

// SlipstreamConfig holds config for slipstream transport.
type SlipstreamConfig struct {
	Cert string `json:"cert"` // path to cert file
	Key  string `json:"key"`  // path to key file
}

// NaiveConfig holds config for naiveproxy transport.
type NaiveConfig struct {
	Email    string `json:"email"`
	DecoyURL string `json:"decoy_url"`
	Port     int    `json:"port"` // typically 443
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

// VayDNSConfig holds config for VayDNS transport (KCP + Curve25519).
type VayDNSConfig struct {
	MTU           int    `json:"mtu"`
	PrivateKey    string `json:"private_key"`             // path to key file
	PublicKey     string `json:"public_key"`              // hex-encoded public key
	IdleTimeout   string `json:"idle_timeout,omitempty"`  // e.g. "10s", "2m"
	KeepAlive     string `json:"keep_alive,omitempty"`    // e.g. "2s"
	Fallback      string `json:"fallback,omitempty"`      // fallback DNS address
	DnsttCompat   bool   `json:"dnstt_compat,omitempty"`  // dnstt wire-format compatibility
	ClientIDSize  int    `json:"clientid_size,omitempty"` // client ID bytes (default 2)
	QueueSize     int    `json:"queue_size,omitempty"`    // KCP queue size (default 512)
	KCPWindowSize int    `json:"kcp_window_size,omitempty"`
	QueueOverflow string `json:"queue_overflow,omitempty"` // "drop" or "block"
	RecordType    string `json:"record_type,omitempty"`    // txt, cname, a, aaaa, mx, ns, srv, null, caa
}

// ValidVayDNSRecordTypes lists the valid DNS record types for VayDNS.
var ValidVayDNSRecordTypes = []string{"txt", "cname", "a", "aaaa", "mx", "ns", "srv", "null", "caa"}

// ResolvedIdleTimeout returns the idle-timeout value, applying defaults.
func (v *VayDNSConfig) ResolvedIdleTimeout() string {
	if v == nil {
		return "10s"
	}
	if v.IdleTimeout != "" {
		return v.IdleTimeout
	}
	if v.DnsttCompat {
		return "2m"
	}
	return "10s"
}

// ResolvedKeepAlive returns the keepalive value, applying defaults.
func (v *VayDNSConfig) ResolvedKeepAlive() string {
	if v == nil {
		return "2s"
	}
	if v.KeepAlive != "" {
		return v.KeepAlive
	}
	if v.DnsttCompat {
		return "10s"
	}
	return "2s"
}

// ResolvedClientIDSize returns the clientid-size flag value, or 0 if omitted (dnstt-compat).
func (v *VayDNSConfig) ResolvedClientIDSize() int {
	if v == nil {
		return 2
	}
	if v.DnsttCompat {
		return 0
	}
	if v.ClientIDSize <= 0 {
		return 2
	}
	return v.ClientIDSize
}

// SoarConfig holds config for Soar transport (X25519 + Ed25519-signed handshake).
//
// Soar's server dials each stream's destination itself and has no per-user
// login. With the direct backend it reaches public destinations on
// AllowPorts, so anyone who finds the zone can use it. With the SSH backend
// it reaches the SSH backend address and nothing else, so access goes
// through SSH logins.
type SoarConfig struct {
	PrivateKey string `json:"private_key"`           // path to key file (base64 Ed25519 seed)
	PublicKey  string `json:"public_key"`            // base64-encoded Ed25519 public key
	AllowPorts string `json:"allow_ports,omitempty"` // direct backend: destination ports, "any" (default "80,443")
}

// DefaultSoarAllowPorts matches soar-server's default destination ports.
const DefaultSoarAllowPorts = "80,443"

// ResolvedAllowPorts returns the allow-ports value, applying defaults.
func (s *SoarConfig) ResolvedAllowPorts() string {
	if s == nil || s.AllowPorts == "" {
		return DefaultSoarAllowPorts
	}
	// No spaces: the value is passed unquoted on the systemd ExecStart line.
	return strings.ReplaceAll(s.AllowPorts, " ", "")
}

// StunTLSConfig holds config for the TLS + WebSocket SSH proxy transport.
// Accepts both raw TLS connections (stunnel-style) and WebSocket upgrades,
// forwarding traffic to the SSH daemon.
type StunTLSConfig struct {
	Cert string `json:"cert"` // path to TLS certificate
	Key  string `json:"key"`  // path to TLS private key
	Port int    `json:"port"` // listen port (typically 443)
	// Front is the tag of a NaiveProxy tunnel whose Caddy carries SSH over
	// WebSocket on Path, on its own port (443) with its real certificate.
	Front string `json:"front,omitempty"`
	Path  string `json:"path,omitempty"`
}

// Fronted reports whether the tunnel is served through a NaiveProxy Caddy.
func (s *StunTLSConfig) Fronted() bool { return s != nil && s.Front != "" }

// VLESSConfig holds config for the built-in VLESS over WebSocket server.
//
// Clients reach it at Domain, normally through a CDN such as Cloudflare
// (orange cloud), so the server's own IP stays hidden and can't be blocked
// by address. Each Rovunet user gets their own VLESS UUID, derived from
// their credentials and the tunnel secret.
type VLESSConfig struct {
	Port       int    `json:"port"`                  // listen port
	Path       string `json:"path"`                  // WebSocket path
	TLS        bool   `json:"tls"`                   // serve TLS (self-signed) instead of plain HTTP
	Cert       string `json:"cert,omitempty"`        // path to TLS certificate (when TLS)
	Key        string `json:"key,omitempty"`         // path to TLS private key (when TLS)
	SecretFile string `json:"secret_file"`           // path to the UUID-derivation secret
	CDN        bool   `json:"cdn"`                   // Domain is proxied by a CDN (Cloudflare orange cloud)
	CDNAddress string `json:"cdn_address,omitempty"` // address clients dial (clean CDN IP); empty = Domain
	BlockPorts string `json:"block_ports,omitempty"` // destination ports refused; empty = DefaultVLESSBlockPorts, "none" = no limit
	// Front is the tag of a NaiveProxy tunnel whose Caddy serves this
	// VLESS path on its own port (443) with its real Let's Encrypt
	// certificate. The VLESS server then listens on localhost only.
	Front string `json:"front,omitempty"`
}

// Fronted reports whether the tunnel is served through a NaiveProxy Caddy.
func (v *VLESSConfig) Fronted() bool { return v != nil && v.Front != "" }

// DefaultVLESSSecret is the server-wide secret behind every VLESS tunnel's
// user UUIDs; it is not inside a tunnel's directory, so removing a tunnel
// doesn't lose it.
const DefaultVLESSSecret = "/etc/rovunet/vless.secret"

// DefaultVLESSBlockPorts keeps users from sending mail straight from the
// server's IP, the usual way a proxy gets the IP blacklisted.
const DefaultVLESSBlockPorts = "25"

// Cloudflare proxies only these ports. A CDN-fronted VLESS tunnel must listen
// on one of them, since Cloudflare reaches the origin on the port the client
// used.
var (
	CloudflareHTTPSPorts = []int{443, 2053, 2083, 2087, 2096, 8443}
	CloudflareHTTPPorts  = []int{80, 8080, 8880, 2052, 2082, 2086, 2095}
)

// ResolvedBlockPorts returns the block-ports value, applying defaults.
func (v *VLESSConfig) ResolvedBlockPorts() string {
	if v == nil || v.BlockPorts == "" {
		return DefaultVLESSBlockPorts
	}
	// No spaces: the value is passed unquoted on the systemd ExecStart line.
	return strings.ReplaceAll(v.BlockPorts, " ", "")
}

// ClientAddress returns the address clients connect to.
func (v *VLESSConfig) ClientAddress(domain string) string {
	if v != nil && v.CDNAddress != "" {
		return v.CDNAddress
	}
	return domain
}

// CloudflarePortOK reports whether Cloudflare can proxy the tunnel's port
// for its TLS mode.
func (v *VLESSConfig) CloudflarePortOK() bool {
	if v == nil {
		return false
	}
	ports := CloudflareHTTPPorts
	if v.TLS {
		ports = CloudflareHTTPSPorts
	}
	for _, p := range ports {
		if p == v.Port {
			return true
		}
	}
	return false
}

// IsDNSTunnel returns true if the transport uses DNS port 53.
func (t *TunnelConfig) IsDNSTunnel() bool {
	switch t.Transport {
	case TransportDNSTT, TransportSlipstream, TransportVayDNS, TransportSoar, TransportExternal:
		return true
	}
	return false
}

// HasManagedService returns true if rovunet manages a systemd service for this tunnel.
func (t *TunnelConfig) HasManagedService() bool {
	return t.IsDNSTunnel() && t.Transport != TransportExternal
}

// ConnectsByAddress reports whether clients reach this tunnel at the
// server's address rather than through a domain.
func (t *TunnelConfig) ConnectsByAddress() bool {
	switch t.Transport {
	case TransportSSH, TransportSOCKS, TransportStunTLS:
		return true
	}
	return false
}

// IsDirectTransport returns true for transports that expose a service directly (no tunnel).
func (t *TunnelConfig) IsDirectTransport() bool {
	switch t.Transport {
	case TransportSSH, TransportSOCKS:
		return true
	}
	return false
}
