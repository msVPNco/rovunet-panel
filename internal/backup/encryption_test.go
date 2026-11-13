package backup

import (
	"reflect"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestEncryptedBackupAuthenticatesBeforeReturningBundle(t *testing.T) {
	cfg, root := testServer(t)
	cfg.ServerID = "stable-server"
	cfg.Users[0].AccountID = "stable-account"
	cfg.Users[0].PeriodID = "stable-period"
	cfg.Users[0].SubToken = "00112233445566778899aabbccddeeff"
	b, err := Create(cfg, root, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	b.Usage = map[string]int64{"alice": 1234567, "bob": 7654321}
	b.UsageHistory = map[string][]byte{"xray-fixture.source": []byte(`{"epoch":"one","value":42}`)}
	plain, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	id, _ := age.GenerateX25519Identity()
	wrong, _ := age.GenerateX25519Identity()
	cipher, err := Encrypt(plain, id.Recipient())
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(cipher) || strings.Contains(cipher, BeginMarker) || strings.Contains(cipher, "stable-account") {
		t.Fatal("plaintext leaked into encrypted backup")
	}
	decoded, err := DecodeEncrypted(cipher, id)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := Decode(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, legacy) {
		t.Fatal("keys, identity or consumption changed across encryption")
	}
	for name, input := range map[string]string{"wrong-key": cipher, "truncated": cipher[:len(cipher)/2], "damaged": strings.Replace(cipher, "AGE ENCRYPTED", "AGE DAMAGED", 1)} {
		t.Run(name, func(t *testing.T) {
			use := id
			if name == "wrong-key" {
				use = wrong
			}
			got, err := DecodeEncrypted(input, use)
			if err == nil || got != nil {
				t.Fatal("unauthenticated/partial backup exposed to restore")
			}
		})
	}
	// Flip ciphertext without damaging the armor syntax, then demand AEAD failure.
	lines := strings.Split(cipher, "\n")
	for n := len(lines) - 3; n > 1; n-- {
		if len(lines[n]) > 8 {
			if lines[n][0] == 'A' {
				lines[n] = "B" + lines[n][1:]
			} else {
				lines[n] = "A" + lines[n][1:]
			}
			break
		}
	}
	if got, err := DecodeEncrypted(strings.Join(lines, "\n"), id); err == nil || got != nil {
		t.Fatal("tampered payload restored")
	}
}
