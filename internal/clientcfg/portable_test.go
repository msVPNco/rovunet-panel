package clientcfg

import (
	"encoding/base64"
	"github.com/anonvector/slipgate/internal/config"
	"os"
	"strings"
	"testing"
)

func TestPortableLosslessFallback(t *testing.T) {
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	tunnel := &config.TunnelConfig{Tag: "soar", Transport: config.TransportSoar, Backend: config.BackendDirect, Domain: "r.example.com", Soar: &config.SoarConfig{PublicKey: testSoarKey}}
	legacy, err := GeneratePortableURI(tunnel, nil, cfg, URIOptions{}, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"veyra"} {
		link, err := GeneratePortableURI(tunnel, nil, cfg, URIOptions{}, format)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimPrefix(link, "veyra://") != strings.TrimPrefix(legacy, "slipnet://") {
			t.Fatal("payload changed")
		}
	}
	for _, format := range []string{"standard", "unknown"} {
		if _, err := GeneratePortableURI(tunnel, nil, cfg, URIOptions{}, format); err == nil {
			t.Fatal("unsupported export accepted")
		}
	}
}

func TestPortableRejectsUntrustedTLS(t *testing.T) {
	tunnel := &config.TunnelConfig{Transport: config.TransportVLESS, Domain: "example.com", VLESS: &config.VLESSConfig{TLS: true, Port: 443}}
	if _, err := GeneratePortableURI(tunnel, nil, &config.Config{}, URIOptions{}, "auto"); err == nil || !strings.Contains(err.Error(), "trusted TLS") {
		t.Fatal(err)
	}
}

func TestRovunetCollectionMetadata(t *testing.T) {
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	tunnel := &config.TunnelConfig{Tag: "soar", Transport: config.TransportSoar, Backend: config.BackendDirect, Domain: "r.example.com", Soar: &config.SoarConfig{PublicKey: testSoarKey}}
	export := func(user string) []string {
		uri, err := GeneratePortableURI(tunnel, nil, cfg, URIOptions{Username: user}, "rovunet")
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "rovunet://"))
		if err != nil {
			t.Fatal(err)
		}
		f := strings.Split(string(raw), "|")
		if len(f) != TotalFieldsV32 || f[0] != "32" {
			t.Fatal("invalid v32 metadata")
		}
		if f[FSoarDiscoveryList] != "soar" {
			t.Fatal("invalid discovery list")
		}
		return f
	}
	a, b := export("alice"), export("other")
	if file := os.Getenv("ROVUNET_ANDROID_FIXTURE"); file != "" {
		if err := os.WriteFile(file, []byte("rovunet://"+base64.StdEncoding.EncodeToString([]byte(strings.Join(a, "|")))), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if a[FCollectionID] == b[FCollectionID] {
		t.Fatal("different users merged")
	}
	if a[FCollectionID] != export("alice")[FCollectionID] {
		t.Fatal("unstable identity")
	}
	cfg.PublicAddress = "192.0.2.2"
	if a[FCollectionID] == export("alice")[FCollectionID] {
		t.Fatal("different servers merged")
	}
}
