package keys

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// VLESSSecretSize is the length of a VLESS tunnel's UUID-derivation secret.
const VLESSSecretSize = 32

// vlessUUIDContext separates VLESS UUID derivation from any other use of the
// secret, so the same bytes can never produce a value meaningful elsewhere.
const vlessUUIDContext = "slipgate-vless-uuid-v1"

// GenerateVLESSSecret writes a fresh random secret (hex) readable only by
// root. The secret turns each user's credentials into their VLESS UUID, so
// the UUID in a shared config reveals nothing about the password.
func GenerateVLESSSecret(path string) error {
	secret := make([]byte, VLESSSecretSize)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("generate vless secret: %w", err)
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(secret)+"\n"), 0600); err != nil {
		return fmt.Errorf("write vless secret: %w", err)
	}
	return nil
}

// LoadVLESSSecret reads a secret written by GenerateVLESSSecret.
func LoadVLESSSecret(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	secret, err := hex.DecodeString(strings.TrimSpace(string(contents)))
	if err != nil {
		return nil, fmt.Errorf("%s: invalid vless secret: %w", path, err)
	}
	if len(secret) < 16 {
		return nil, fmt.Errorf("%s: vless secret too short (%d bytes)", path, len(secret))
	}
	return secret, nil
}

// VLESSUserID derives a user's 16-byte VLESS ID from the tunnel secret and
// the user's SlipGate credentials. Changing the password or removing the user
// revokes the ID; nobody without the secret can link an ID to a password.
// The bytes carry RFC 4122 version 4 and variant bits so every client that
// validates UUIDs accepts them.
func VLESSUserID(secret []byte, username, password string) [16]byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(vlessUUIDContext))
	mac.Write([]byte{0})
	mac.Write([]byte(username))
	mac.Write([]byte{0})
	mac.Write([]byte(password))
	sum := mac.Sum(nil)

	var id [16]byte
	copy(id[:], sum[:16])
	id[6] = id[6]&0x0f | 0x40 // version 4
	id[8] = id[8]&0x3f | 0x80 // RFC 4122 variant
	return id
}

// FormatUUID renders 16 bytes in canonical 8-4-4-4-12 form.
func FormatUUID(id [16]byte) string {
	h := hex.EncodeToString(id[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// ParseUUID parses a UUID with or without dashes.
func ParseUUID(s string) ([16]byte, error) {
	var id [16]byte
	b, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(s), "-", ""))
	if err != nil || len(b) != 16 {
		return id, fmt.Errorf("invalid uuid %q", s)
	}
	copy(id[:], b)
	return id, nil
}
