// Package releaseauth authenticates release bytes against independently
// provisioned Ed25519 public keys. A checksum from the download site is not a
// trust anchor. No production signing identity is invented by this package.
package releaseauth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
)

const MaxManifestSize = 1 << 20

//go:embed production-keys.json
var productionKeys []byte

type Key struct {
	ID      string `json:"id"`
	SPKI    string `json:"spki"`
	Purpose string `json:"purpose"`
}

type Keyring struct {
	Schema int   `json:"schema"`
	Keys   []Key `json:"keys"`
}

type Asset struct {
	Name         string `json:"name"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	Kind         string `json:"kind"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	Source       string `json:"source"`
	SourceCommit string `json:"source_commit"`
	License      string `json:"license"`
}

type Manifest struct {
	Schema       int     `json:"schema"`
	Environment  string  `json:"environment"`
	KeyID        string  `json:"key_id"`
	Repository   string  `json:"repository"`
	Tag          string  `json:"tag"`
	SourceCommit string  `json:"source_commit"`
	Assets       []Asset `json:"assets"`
}

type Verifier struct {
	keys        map[string]ed25519.PublicKey
	environment string
}

var (
	ErrNoTrustAnchor = errors.New("production release signing identity has not been provisioned; unsigned downloads are refused")
	assetName        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)
	tagName          = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$`)
)

// Production returns only the keys compiled into the reviewed panel. It does
// not trust environment variables, a downloaded key, or any test key store.
func Production() (*Verifier, error) { return NewVerifier(productionKeys, "production") }

// NewVerifier is used by offline validation and isolated tests. Production
// download paths always obtain their verifier from Production.
func NewVerifier(raw []byte, environment string) (*Verifier, error) {
	if environment != "production" && environment != "test" {
		return nil, errors.New("invalid trust environment")
	}
	var ring Keyring
	if err := decodeStrict(raw, &ring); err != nil {
		return nil, err
	}
	if ring.Schema != 1 {
		return nil, errors.New("unsupported trust keyring")
	}
	v := &Verifier{keys: map[string]ed25519.PublicKey{}, environment: environment}
	for _, k := range ring.Keys {
		if k.Purpose != environment {
			return nil, errors.New("trust key purpose differs from build environment")
		}
		der, err := base64.StdEncoding.DecodeString(k.SPKI)
		if err != nil {
			return nil, errors.New("invalid public key encoding")
		}
		sum := sha256.Sum256(der)
		if k.ID != hex.EncodeToString(sum[:]) {
			return nil, errors.New("public key fingerprint mismatch")
		}
		pub, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			return nil, errors.New("invalid release public key")
		}
		ed, ok := pub.(ed25519.PublicKey)
		if !ok {
			return nil, errors.New("release key must be Ed25519")
		}
		if _, exists := v.keys[k.ID]; exists {
			return nil, errors.New("duplicate release key")
		}
		v.keys[k.ID] = ed
	}
	if len(v.keys) == 0 {
		return nil, ErrNoTrustAnchor
	}
	return v, nil
}

func decodeStrict(raw []byte, dst any) error {
	if len(raw) == 0 || len(raw) > MaxManifestSize {
		return errors.New("invalid manifest/keyring size")
	}
	// encoding/json otherwise accepts duplicate members with last-value wins.
	// Authenticate one unambiguous document across bootstrap and Go readers.
	if err := uniqueMembers(json.NewDecoder(bytes.NewReader(raw)), 0); err != nil {
		return errors.New("ambiguous or excessively nested release document")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return errors.New("invalid release manifest/keyring")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing release manifest data")
	}
	return nil
}

func uniqueMembers(d *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("nesting limit")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate member")
			}
			seen[name] = true
			if err := uniqueMembers(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueMembers(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid delimiter")
	}
	_, err = d.Token()
	return err
}

// Verify checks the signature on the original bytes, then their schema and
// exact release identity. Missing signatures and unknown keys fail closed.
func (v *Verifier) Verify(raw, signature []byte, repository, tag string) (*Manifest, error) {
	var m Manifest
	if err := decodeStrict(raw, &m); err != nil {
		return nil, err
	}
	key := v.keys[m.KeyID]
	if len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, raw, signature) {
		return nil, errors.New("release manifest signature is invalid or its key is untrusted")
	}
	if m.Schema != 1 || m.Environment != v.environment || m.Repository != repository || !tagName.MatchString(m.Tag) || (tag != "" && m.Tag != tag) {
		return nil, errors.New("release manifest identity does not match requested release")
	}
	if !hexBytes(m.SourceCommit, 20) {
		return nil, errors.New("manifest source commit is invalid")
	}
	if len(m.Assets) == 0 || len(m.Assets) > 256 {
		return nil, errors.New("invalid manifest assets")
	}
	seen := map[string]bool{}
	for _, a := range m.Assets {
		if !assetName.MatchString(a.Name) || seen[a.Name] || a.Size <= 0 || a.Size > 512<<20 || !hexBytes(a.SHA256, 32) || a.Source == "" || a.SourceCommit == "" || a.License == "" {
			return nil, errors.New("manifest contains an incomplete or duplicate asset")
		}
		if a.Kind != "executable" && a.Kind != "data" && a.Kind != "installer" {
			return nil, errors.New("invalid manifest asset kind")
		}
		if a.Kind == "executable" && (a.OS != "linux" || (a.Arch != "amd64" && a.Arch != "arm64")) {
			return nil, errors.New("unsupported executable architecture")
		}
		seen[a.Name] = true
	}
	return &m, nil
}

func hexBytes(s string, length int) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == length && s == hex.EncodeToString(b)
}

func (m *Manifest) Find(name, goos, arch string) (Asset, error) {
	for _, a := range m.Assets {
		if a.Name == name {
			if a.OS != "" && a.OS != goos || a.Arch != "" && a.Arch != arch {
				return Asset{}, errors.New("release asset architecture does not match this host")
			}
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("authenticated manifest has no entry for %s", name)
}
