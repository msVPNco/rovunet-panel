package clientcfg

import "github.com/msVPNco/rovunet-panel/internal/config"

// v17 field positions (pipe-delimited, 38 fields).
// Matches the SlipNet Android app's ConfigImporter/ConfigExporter.
const (
	FVersion            = 0  // "17"
	FTunnelType         = 1  // dnstt, sayedns, dnstt_ssh, sayedns_ssh, naive, naive_ssh, ss, slipstream_ssh
	FName               = 2  // profile name
	FDomain             = 3  // tunnel domain
	FResolvers          = 4  // comma-separated "host:port:auth"
	FAuthMode           = 5  // "0" or "1"
	FKeepAlive          = 6  // "5000"
	FCongestionControl  = 7  // "bbr"
	FTCPListenPort      = 8  // "1080"
	FTCPListenHost      = 9  // "127.0.0.1"
	FGSOEnabled         = 10 // "0"
	FPublicKey          = 11 // hex-encoded Curve25519 public key
	FSOCKSUser          = 12 // SOCKS5 username
	FSOCKSPass          = 13 // SOCKS5 password
	FSSHEnabled         = 14 // "0" or "1"
	FSSHUser            = 15 // SSH username
	FSSHPass            = 16 // SSH password
	FSSHPort            = 17 // "22"
	FFwdDNSThroughSSH   = 18 // "0" (deprecated)
	FSSHHost            = 19 // SSH host address
	FUseServerDNS       = 20 // "0" (deprecated)
	FDoHURL             = 21 // DoH URL
	FDNSTransport       = 22 // "udp"
	FSSHAuthType        = 23 // "password"
	FSSHPrivateKey      = 24 // base64-encoded
	FSSHKeyPassphrase   = 25 // base64-encoded
	FTorBridgeLines     = 26 // base64-encoded
	FDNSTTAuthoritative = 27 // "0" or "1"
	FNaivePort          = 28 // "443"
	FNaiveUser          = 29 // NaiveProxy username
	FNaivePass          = 30 // NaiveProxy password (base64)
	FIsLocked           = 31 // "0"
	FLockPasswordHash   = 32 // ""
	FExpirationDate     = 33 // "0"
	FAllowSharing       = 34 // "0"
	FBoundDeviceId      = 35 // ""
	FResolversHidden    = 36 // "0" (v17)
	FHiddenResolvers    = 37 // "" (v17)
	FNoizDNSStealth     = 38 // "0" or "1" (v18)
	FDNSPayloadSize     = 39 // integer (v18)
	FSOCKS5ServerPort   = 40 // integer, default 1080 (v18)
	// v19: VayDNS fields (41-44)
	FVayDNSDnsttCompat = 41
	FVayDNSRecordType  = 42
	FVayDNSMaxQnameLen = 43
	FVayDNSRps         = 44
	// v20: VayDNS advanced (45-49)
	FVayDNSIdleTimeout  = 45
	FVayDNSKeepalive    = 46
	FVayDNSUdpTimeout   = 47
	FVayDNSMaxNumLabels = 48
	FVayDNSClientIdSize = 49
	// v21: SSH over TLS + HTTP proxy + WebSocket (50-58)
	FSSHTlsEnabled          = 50
	FSSHTlsSni              = 51
	FSSHHttpProxyHost       = 52
	FSSHHttpProxyPort       = 53
	FSSHHttpProxyCustomHost = 54
	FSSHWsEnabled           = 55
	FSSHWsPath              = 56
	FSSHWsUseTls            = 57
	FSSHWsCustomHost        = 58
	// v22: SSH payload (raw prefix for DPI bypass)
	FSSHPayload = 59
	// v23-v24: multi-resolver mode
	FResolverMode  = 60
	FRRSpreadCount = 61
	// v25-v28: VLESS + SNI fragmentation
	FVlessUUID           = 62
	FVlessSecurity       = 63
	FVlessTransport      = 64
	FVlessWsPath         = 65
	FCdnIP               = 66
	FCdnPort             = 67
	FSniFragmentEnabled  = 68
	FSniFragmentStrategy = 69
	FSniFragmentDelayMs  = 70
	FLegacySni           = 71
	FCHPadding           = 72
	FWSHeaderObfuscation = 73
	FWSPadding           = 74
	FSniSpoofTTL         = 75
	FFakeDecoyHost       = 76
	FTCPMaxSeg           = 77
	FVlessSNI            = 78
	// v29: Soar
	FSoarDiscovery = 79
	FSoarCountry   = 80
	FSoarTCPMode   = 81
	TotalFields    = 82

	// V22Fields is the length of a v22 URI. Transports the v22 app already
	// understands are still exported as v22, so their URIs don't change.
	V22Fields = 60

	// V28Fields is the length of a v28 URI, the first with the dedicated
	// VLESS SNI slot. VLESS configs use it so apps without Soar import them.
	V28Fields = FVlessSNI + 1
)

// Client modes for DNSTT transport (server is the same, client behavior differs).
const (
	ClientModeDNSTT   = "dnstt"
	ClientModeNoizDNS = "noizdns"
)

// TunnelTypeMap maps transport+clientMode+backend to slipnet:// field[1] value.
var TunnelTypeMap = map[string]map[string]map[string]string{
	config.TransportDNSTT: {
		ClientModeDNSTT: {
			config.BackendSOCKS: "dnstt",
			config.BackendSSH:   "dnstt_ssh",
		},
		ClientModeNoizDNS: {
			config.BackendSOCKS: "sayedns",
			config.BackendSSH:   "sayedns_ssh",
		},
	},
	config.TransportSlipstream: {
		"": {
			config.BackendSOCKS: "ss",
			config.BackendSSH:   "slipstream_ssh",
		},
	},
	config.TransportVayDNS: {
		"": {
			config.BackendSOCKS: "vaydns",
			config.BackendSSH:   "vaydns_ssh",
		},
	},
	config.TransportSoar: {
		"": {
			config.BackendDirect: "soar",
			config.BackendSSH:    "soar_ssh",
		},
	},
	config.TransportNaive: {
		"": {
			config.BackendSOCKS: "naive",
			config.BackendSSH:   "naive_ssh",
		},
	},
	config.TransportStunTLS: {
		"": {
			config.BackendSSH: "ssh",
		},
	},
	config.TransportVLESS: {
		"": {
			config.BackendDirect: "vless",
		},
	},
	config.TransportSSH: {
		"": {
			config.BackendSSH: "ssh",
		},
	},
	config.TransportSOCKS: {
		"": {
			config.BackendSOCKS: "socks5",
		},
	},
}

// GetTunnelType returns the slipnet:// type string.
func GetTunnelType(transport, backend, clientMode string) string {
	modes, ok := TunnelTypeMap[transport]
	if !ok {
		return "unknown"
	}
	if transport != config.TransportDNSTT {
		clientMode = ""
	}
	if m, ok := modes[clientMode]; ok {
		if t, ok := m[backend]; ok {
			return t
		}
	}
	return "unknown"
}

// Native v30-v32 extensions. TotalFields remains the legacy v29 wire length.
const (
	FSoarDiscoveryList   = 82
	FSoarDiscoveryCustom = 83
	FCoreLink            = 84
	FCollectionID        = 85
	FCollectionName      = 86
	TotalFieldsV32       = 87
)

// FCredentialsV33 (app-side: ConfigImporter.parseProfileV33) carries
// SOCKS/SSH credentials as one base64(JSON) field — {"socksUser",
// "socksPass", "sshUser", "sshPass"} — instead of relying solely on the
// plain FSOCKSUser/FSOCKSPass/FSSHUser/FSSHPass fields below, which are
// joined into the wire string with "|" and so can't hold a credential
// containing "|" without desynchronizing every field after it.
// TotalFieldsV33 is TotalFieldsV32 + 1 field (index 87).
const (
	FCredentialsV33 = 87
	TotalFieldsV33  = 88
)
