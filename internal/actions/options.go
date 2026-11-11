package actions

// Shared select options used across multiple actions.

// TransportOptions is the full list shown in `tunnel add`.
var TransportOptions = []SelectOption{
	{Value: "dnstt", Label: "DNSTT / NoizDNS — DNS tunnel"},
	{Value: "slipstream", Label: "Slipstream — QUIC DNS tunnel"},
	{Value: "vaydns", Label: "VayDNS — KCP DNS tunnel"},
	{Value: "soar", Label: "Soar — multipath DNS tunnel (soar-client compatible)"},
	{Value: "naive", Label: "NaiveProxy — HTTPS proxy with Caddy"},
	{Value: "stuntls", Label: "StunTLS — SSH over TLS + WebSocket proxy"},
	{Value: "vless", Label: "VLESS — VLESS over WebSocket, Cloudflare CDN ready"},
	{Value: "external", Label: "External — Route DNS to a custom port"},
	{Value: "direct-ssh", Label: "SSH — Direct SSH tunnel"},
	{Value: "direct-socks5", Label: "SOCKS5 — Direct SOCKS5 proxy (unencrypted: open networks only)"},
	{Value: "xray-reality", Label: "VLESS + REALITY — Xray core, fast, no domain or certificate needed (recommended)"},
	{Value: "xray-xhttp", Label: "VLESS + XHTTP — Xray core, HTTPS-like over TCP, uses NaiveProxy's domain and certificate (recommended)"},
	{Value: "xray-hysteria2", Label: "Hysteria 2 — Xray core, QUIC over UDP, uses NaiveProxy's certificate (blocked on some networks)"},
	{Value: "xray-wireguard", Label: "WireGuard — Xray core, UDP, one configuration per user (blocked on some networks)"},
}

// QuickTransportOptions is the subset shown in the quick wizard.
// External is excluded because it routes to a user-managed service that
// wouldn't exist yet during initial setup.
var QuickTransportOptions = []SelectOption{
	{Value: "dnstt", Label: "DNSTT / NoizDNS — DNS tunnel"},
	{Value: "slipstream", Label: "Slipstream — QUIC DNS tunnel"},
	{Value: "vaydns", Label: "VayDNS — KCP DNS tunnel"},
	{Value: "soar", Label: "Soar — multipath DNS tunnel (soar-client compatible)"},
	{Value: "naive", Label: "NaiveProxy — HTTPS proxy with Caddy"},
	{Value: "stuntls", Label: "StunTLS — SSH over TLS + WebSocket proxy"},
	{Value: "direct-ssh", Label: "SSH — Direct SSH tunnel"},
	{Value: "direct-socks5", Label: "SOCKS5 — Direct SOCKS5 proxy"},
}

// InstallTransportOptions is the list `rovunet install` shows: the quick
// wizard's plus VLESS, which install sets up with no further questions than
// its domain.
var InstallTransportOptions = []SelectOption{
	{Value: "dnstt", Label: "DNSTT / NoizDNS — DNS tunnel"},
	{Value: "slipstream", Label: "Slipstream — QUIC DNS tunnel"},
	{Value: "vaydns", Label: "VayDNS — KCP DNS tunnel"},
	{Value: "soar", Label: "Soar — multipath DNS tunnel (soar-client compatible)"},
	{Value: "naive", Label: "NaiveProxy — HTTPS proxy with Caddy"},
	{Value: "stuntls", Label: "StunTLS — SSH over TLS + WebSocket proxy"},
	{Value: "vless", Label: "VLESS — over WebSocket behind Cloudflare (ready with just a domain)"},
	{Value: "direct-ssh", Label: "SSH — Direct SSH tunnel"},
	{Value: "direct-socks5", Label: "SOCKS5 — Direct SOCKS5 proxy"},
	{Value: "xray-reality", Label: "VLESS + REALITY — Xray core, no domain or certificate needed"},
	{Value: "xray-wireguard", Label: "WireGuard — Xray core, one configuration per user"},
}

func init() {
	managed := []SelectOption{
		{Value: "xray-amneziawg", Label: "AmneziaWG — official userspace engine; unlimited/time accounts; Android"},
		{Value: "xray-ss2022", Label: "Shadowsocks 2022 — separate account keys; Android; deprecated in pinned Xray"},
		{Value: "xray-trojan", Label: "Trojan TLS — valid domain certificate; deprecated in pinned Xray"},
		{Value: "xray-vlessenc", Label: "VLESS Encryption — ML-KEM-768 authentication, raw TCP"},
	}
	TransportOptions = append(TransportOptions, managed...)
	InstallTransportOptions = append(InstallTransportOptions, managed...)
}

// ClientModeOptions is used during `tunnel share` to pick the slipnet:// URL type.
var ClientModeOptions = []SelectOption{
	{Value: "dnstt", Label: "DNSTT — Classic DNS tunnel client"},
	{Value: "noizdns", Label: "NoizDNS — DNS tunnel with DPI evasion"},
}

// SoarBackendOptions replaces BackendOptions for Soar, which dials
// destinations itself (direct) or reaches only the SSH backend.
var SoarBackendOptions = []SelectOption{
	{Value: "direct", Label: "Direct — no login; server reaches public sites (Lantern-compatible)"},
	{Value: "ssh", Label: "SSH — per-user logins (Rovunet app)"},
	{Value: "both", Label: "Both — Direct + SSH"},
}

var BackendOptions = []SelectOption{
	{Value: "socks", Label: "SOCKS — SOCKS5 proxy"},
	{Value: "ssh", Label: "SSH — SSH tunnel"},
	{Value: "both", Label: "Both — SOCKS5 + SSH tunnel"},
}

var RouterModeOptions = []SelectOption{
	{Value: "single", Label: "Single — one active tunnel at a time"},
	{Value: "multi", Label: "Multi — domain-based routing to multiple tunnels"},
}
