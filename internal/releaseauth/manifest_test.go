package releaseauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Verifier, Manifest, ed25519.PrivateKey, []byte) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(pub)
	id := sha256.Sum256(der)
	k := Key{ID: hex.EncodeToString(id[:]), SPKI: base64.StdEncoding.EncodeToString(der), Purpose: "test"}
	ring, _ := json.Marshal(Keyring{Schema: 1, Keys: []Key{k}})
	v, err := NewVerifier(ring, "test")
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{Schema: 1, Environment: "test", KeyID: k.ID, Repository: "example/panel", Tag: "v1.2.3", SourceCommit: strings.Repeat("a", 40), Assets: []Asset{{Name: "panel-linux-amd64", OS: "linux", Arch: "amd64", Kind: "executable", Size: 64, SHA256: strings.Repeat("b", 64), Source: "https://github.com/example/panel", SourceCommit: strings.Repeat("a", 40), License: "MIT"}}}
	return v, m, priv, ring
}

func TestAuthenticatedManifestIdentityAndTamper(t *testing.T) {
	v, m, priv, _ := fixture(t)
	raw, _ := json.Marshal(m)
	sig := ed25519.Sign(priv, raw)
	got, err := v.Verify(raw, sig, m.Repository, m.Tag)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := got.Find(m.Assets[0].Name, "linux", "arm64"); err == nil {
		t.Fatal("wrong architecture accepted")
	}
	for _, tc := range []struct {
		name      string
		body, sig []byte
		repo, tag string
	}{
		{"tampered", append(append([]byte{}, raw...), '\n'), sig, m.Repository, m.Tag},
		{"missing signature", raw, nil, m.Repository, m.Tag},
		{"wrong signature", raw, make([]byte, 64), m.Repository, m.Tag},
		{"wrong repository", raw, sig, "other/panel", m.Tag},
		{"wrong release", raw, sig, m.Repository, "v1.2.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.Verify(tc.body, tc.sig, tc.repo, tc.tag); err == nil {
				t.Fatal("untrusted manifest accepted")
			}
		})
	}
	m.Assets = append(m.Assets, m.Assets[0])
	raw, _ = json.Marshal(m)
	if _, err := v.Verify(raw, ed25519.Sign(priv, raw), m.Repository, m.Tag); err == nil {
		t.Fatal("duplicate asset accepted")
	}
}

func TestSignedDuplicateManifestMemberIsRejected(t *testing.T) {
	v, m, key, _ := fixture(t)
	raw, _ := json.Marshal(m)
	raw = []byte(strings.Replace(string(raw), `"schema":1`, `"schema":1,"schema":1`, 1))
	if _, err := v.Verify(raw, ed25519.Sign(key, raw), m.Repository, m.Tag); err == nil {
		t.Fatal("signed document with ambiguous members accepted")
	}
}

func TestTestAuthorityCannotEnterProduction(t *testing.T) {
	_, m, priv, ring := fixture(t)
	if _, err := NewVerifier(ring, "production"); err == nil {
		t.Fatal("test key accepted as production authority")
	}
	p, err := Production()
	if err != nil {
		t.Fatal("owner-provisioned production root is missing or invalid", err)
	}
	rawTest, _ := json.Marshal(m)
	if _, err := p.Verify(rawTest, ed25519.Sign(priv, rawTest), m.Repository, m.Tag); err == nil {
		t.Fatal("production trusts generated lab authority")
	}
	var r Keyring
	json.Unmarshal(ring, &r)
	r.Keys[0].Purpose = "production"
	ring, _ = json.Marshal(r)
	p, err = NewVerifier(ring, "production")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(m)
	if _, err := p.Verify(raw, ed25519.Sign(priv, raw), m.Repository, m.Tag); err == nil {
		t.Fatal("test release accepted in production")
	}
}
