package coreplan

import "github.com/msVPNco/rovunet-panel/internal/protocols"

// IsShadowsocks includes the original AES-256 core and the two independently
// provisioned compatibility variants. Each keeps its own protocol identity.
func IsShadowsocks(protocol string) bool { return protocols.IsShadowsocks(protocol) }

// Shadowsocks 2022 AES methods use a server key plus a separate key per user.
// Classic AEAD ChaCha20 supports independent users without a server key; it
// must not be confused with the different, single-key SS2022 ChaCha20 method.
func shadowsocksCipher(protocol string) (method string, keySize int, serverKey bool) {
	switch protocol {
	case "ss2022":
		return "2022-blake3-aes-256-gcm", 32, true
	case "ss2022-aes128":
		return "2022-blake3-aes-128-gcm", 16, true
	case "ss-aead-chacha20":
		return "chacha20-ietf-poly1305", 32, false
	default:
		return "", 0, false
	}
}
