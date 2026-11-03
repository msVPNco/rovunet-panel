package clientcfg

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/keys"
)

func testVLESSTunnel(t *testing.T, mutate func(*config.VLESSConfig)) *config.TunnelConfig {
	t.Helper()
	secretFile := filepath.Join(t.TempDir(), "vless.secret")
	if err := keys.GenerateVLESSSecret(secretFile); err != nil {
		t.Fatal(err)
	}
	v := &config.VLESSConfig{
		Port: 8443, Path: "/k9x2", TLS: true, Cert: "c", Key: "k",
		SecretFile: secretFile, CDN: true,
	}
	if mutate != nil {
		mutate(v)
	}
	return &config.TunnelConfig{
		Tag: "vless", Transport: config.TransportVLESS, Backend: config.BackendDirect,
		Domain: "cdn.example.com", Enabled: true, VLESS: v,
	}
}

// TestGenerateURIVLESS: a v28 link the SlipNet importer reads into a VLESS
// profile (ConfigImporter.parseProfileV25..V28), with the user's derived UUID
// and no password or origin address in it.
func TestGenerateURIVLESS(t *testing.T) {
	cfg := &config.Config{Backends: config.DefaultBackends()}
	tunnel := testVLESSTunnel(t, func(v *config.VLESSConfig) { v.CDNAddress = "104.16.1.1" })
	uri, err := GenerateURI(tunnel, nil, cfg, URIOptions{Username: "alice", Password: "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	f := decodeFields(t, uri)
	if len(f) != V28Fields || f[FVersion] != "28" || f[FTunnelType] != "vless" {
		t.Fatalf("got %d fields, version %q, type %q", len(f), f[FVersion], f[FTunnelType])
	}
	secret, _ := keys.LoadVLESSSecret(tunnel.VLESS.SecretFile)
	wantID := keys.FormatUUID(keys.VLESSUserID(secret, "alice", "s3cret"))
	checks := map[int]string{
		FDomain: "cdn.example.com", FVlessUUID: wantID, FVlessSecurity: "tls", FVlessTransport: "ws",
		FVlessWsPath: "/k9x2", FCdnIP: "104.16.1.1", FCdnPort: "8443", FVlessSNI: "",
		FSOCKSUser: "", FSOCKSPass: "", FSSHEnabled: "0", FSSHHost: "",
	}
	for i, want := range checks {
		if f[i] != want {
			t.Errorf("field %d = %q, want %q", i, f[i], want)
		}
	}
	if strings.Contains(strings.Join(f, "|"), "s3cret") {
		t.Error("link carries the user's password")
	}
	if _, err := GenerateURI(tunnel, nil, cfg, URIOptions{}); err == nil {
		t.Error("link without a user accepted")
	}
}

func TestGenerateVLESSLink(t *testing.T) {
	tunnel := testVLESSTunnel(t, nil)
	link, err := GenerateVLESSLink(tunnel, "alice", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "vless" || u.Hostname() != "cdn.example.com" || u.Port() != "8443" || u.Fragment != "vless-alice" {
		t.Fatalf("link %s", link)
	}
	if _, err := keys.ParseUUID(u.User.Username()); err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"type": "ws", "security": "tls", "host": "cdn.example.com", "sni": "cdn.example.com",
		"path": "/k9x2", "encryption": "none", "allowInsecure": "",
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}

	direct := testVLESSTunnel(t, func(v *config.VLESSConfig) { v.CDN = false; v.CDNAddress = "2001:4860::1" })
	link, _ = GenerateVLESSLink(direct, "alice", "s3cret")
	if !strings.Contains(link, "@[2001:4860::1]:8443?") || !strings.Contains(link, "allowInsecure=1") {
		t.Errorf("direct link %s", link)
	}

	plain := testVLESSTunnel(t, func(v *config.VLESSConfig) { v.TLS = false; v.Port = 8080 })
	link, _ = GenerateVLESSLink(plain, "alice", "s3cret")
	if u, _ := url.Parse(link); u.Query().Get("security") != "none" || u.Query().Get("sni") != "" {
		t.Errorf("plain link %s", link)
	}
}

// TestVLESSUUIDChangesWithPassword: changing a password revokes the old UUID.
func TestVLESSUUIDChangesWithPassword(t *testing.T) {
	tunnel := testVLESSTunnel(t, nil)
	a, _ := VLESSUserUUID(tunnel, "alice", "one")
	b, _ := VLESSUserUUID(tunnel, "alice", "two")
	c, _ := VLESSUserUUID(tunnel, "alice", "one")
	if a == b || a != c {
		t.Fatalf("uuids %s %s %s", a, b, c)
	}
	// Version 4, RFC 4122 variant
	if a[14] != '4' || !strings.ContainsRune("89ab", rune(a[19])) {
		t.Errorf("uuid %s lacks v4 bits", a)
	}
}
