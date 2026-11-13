package backup

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// MaxEncodedSize bounds both old armored plaintext and encrypted input before
// decompression. Decode separately retains its 32 MiB decompression bound.
const MaxEncodedSize = 64 << 20

// Encrypt uses the maintained age format and its authenticated streaming
// encryption. The payload remains the existing, validated backup format.
func Encrypt(text string, recipients ...age.Recipient) (string, error) {
	if len(text) > MaxEncodedSize {
		return "", errors.New("backup is too large")
	}
	if _, err := Decode(text); err != nil {
		return "", err
	}
	if len(recipients) == 0 {
		return "", errors.New("backup encryption requires a recipient")
	}
	var out bytes.Buffer
	armored := armor.NewWriter(&out)
	w, err := age.Encrypt(armored, recipients...)
	if err != nil {
		return "", errors.New("could not initialize backup encryption")
	}
	if _, err = w.Write([]byte(text)); err != nil {
		return "", errors.New("could not encrypt backup")
	}
	if err = w.Close(); err != nil {
		return "", errors.New("could not finish authenticated backup")
	}
	if err = armored.Close(); err != nil {
		return "", err
	}
	if out.Len() > MaxEncodedSize {
		return "", errors.New("encrypted backup is too large")
	}
	return out.String(), nil
}

func IsEncrypted(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), armor.Header) || strings.HasPrefix(text, "age-encryption.org/v1\n")
}

// DecodeEncrypted authenticates the entire ciphertext before returning any
// bundle for restore. Partial plaintext is never exposed to restoration code.
func DecodeEncrypted(text string, identities ...age.Identity) (*Bundle, error) {
	if len(text) > MaxEncodedSize {
		return nil, errors.New("backup is too large")
	}
	if !IsEncrypted(text) {
		return nil, errors.New("not an encrypted age backup")
	}
	var in io.Reader = strings.NewReader(text)
	if strings.HasPrefix(strings.TrimSpace(text), armor.Header) {
		in = armor.NewReader(in)
	}
	r, err := age.Decrypt(in, identities...)
	if err != nil {
		return nil, errors.New("backup could not be decrypted: wrong key, damaged data, or unsupported encryption")
	}
	plain, err := io.ReadAll(io.LimitReader(r, MaxEncodedSize+1))
	if err != nil {
		return nil, errors.New("encrypted backup authentication failed; nothing was restored")
	}
	if len(plain) > MaxEncodedSize {
		return nil, errors.New("decrypted backup is too large")
	}
	return Decode(string(plain))
}
