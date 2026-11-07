package coreplan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDistinctProfiles(t *testing.T) {
	for _, protocol := range []string{"reality", "xhttp", "hysteria2", "wireguard"} {
		b, err := Generate(Options{Protocol: protocol, Host: "example.org", SNI: "example.org", RealityTarget: "example.org:443", Port: 8443, Devices: 300, Cert: "cert.pem", Key: "key.pem"})
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(b.Server) || len(b.Clients) != 300 {
			t.Fatal("invalid bundle")
		}
		seen := map[string]bool{}
		for _, c := range b.Clients {
			if seen[c] {
				t.Fatal("duplicate credentials")
			}
			seen[c] = true
			if strings.Contains(c, "server.json") {
				t.Fatal("bad profile")
			}
		}
	}
}
func TestInvalid(t *testing.T) {
	if _, err := Generate(Options{Protocol: "reality", Host: "x", Port: 0, Devices: 1}); err == nil {
		t.Fatal("accepted invalid port")
	}
}

// Feed the generated file itself to the pinned runtime, not a hand-built lookalike.
func TestGeneratedConfigWithXray(t *testing.T) {
	binary := os.Getenv("XRAY")
	if binary == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("XRAY required in CI")
		}
		t.Skip("set XRAY for runtime validation")
	}
	dir := t.TempDir()
	cert, key := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if output, err := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-subj", "/CN=example.org", "-keyout", key, "-out", cert).CombinedOutput(); err != nil {
		t.Fatalf("certificate: %v %s", err, output)
	}
	for _, protocol := range []string{"reality", "xhttp", "hysteria2", "wireguard"} {
		t.Run(protocol, func(t *testing.T) {
			b, err := Generate(Options{Protocol: protocol, Host: "example.org", SNI: "example.org", RealityTarget: "example.org:443", Port: 8443, Devices: 3, Cert: cert, Key: key})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, protocol+".json")
			if err = os.WriteFile(path, b.Server, 0600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command(binary, "run", "-test", "-c", path).CombinedOutput(); err != nil {
				t.Fatalf("generated config rejected: %v %s", err, output)
			}
		})
	}
}

// TestWriteAppFixture writes one real client profile per protocol for the
// Android app's CI, which feeds them to its own importer (CoreTest). Records
// are separated by a line holding only "%%" (WireGuard files span lines).
func TestWriteAppFixture(t *testing.T) {
	path := os.Getenv("ROVUNET_CORE_FIXTURE")
	if path == "" {
		t.Skip("set ROVUNET_CORE_FIXTURE to write the app fixture")
	}
	var records []string
	for _, protocol := range []string{"reality", "xhttp", "hysteria2", "wireguard"} {
		b, err := Generate(Options{Protocol: protocol, Host: "example.org", SNI: "example.org", RealityTarget: "example.org:443", Port: 8443, Devices: 1, Cert: "cert.pem", Key: "key.pem"})
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, strings.TrimSpace(b.Clients[0]))
	}
	if err := os.WriteFile(path, []byte(strings.Join(records, "\n%%\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}
