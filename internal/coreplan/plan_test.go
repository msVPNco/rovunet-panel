package coreplan

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestDistinctProfiles(t *testing.T) {
	for _, protocol := range []string{"reality", "xhttp", "hysteria2", "wireguard", "ss2022", "trojan", "vlessenc"} {
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
	for _, protocol := range []string{"reality", "xhttp", "hysteria2", "wireguard", "ss2022", "trojan", "vlessenc"} {
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
	for _, protocol := range []string{"reality", "xhttp", "hysteria2", "wireguard", "ss2022", "trojan", "vlessenc"} {
		b, err := Generate(Options{Protocol: protocol, Host: "example.org", SNI: "example.org", RealityTarget: "example.org:443", Port: 8443, Devices: 1, Cert: "cert.pem", Key: "key.pem"})
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, strings.TrimSpace(b.Clients[0]))
	}
	if err := os.WriteFile(path, []byte(strings.Join(records, "\n%%\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// The same profiles as users configs prints them: wrapped as native
	// "xray" profiles in one user's collection.
	native := os.Getenv("ROVUNET_CORE_NATIVE_FIXTURE")
	if native == "" {
		return
	}
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	var wrapped []string
	for i, r := range records {
		uri, err := clientcfg.GenerateCoreURI(fmt.Sprintf("core%d", i+1), r, cfg, clientcfg.URIOptions{Username: "alice"})
		if err != nil {
			t.Fatal(err)
		}
		wrapped = append(wrapped, uri)
	}
	if err := os.WriteFile(native, []byte(strings.Join(wrapped, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateForIsStablePerClient(t *testing.T) {
	for _, protocol := range []string{"reality", "xhttp", "hysteria2", "wireguard", "ss2022", "trojan", "vlessenc"} {
		o := Options{Protocol: protocol, Host: "example.org", SNI: "example.org", RealityTarget: "example.org:443", Port: 8443, Cert: "c.pem", Key: "k.pem"}
		st, err := NewState()
		if err != nil {
			t.Fatal(err)
		}
		alice, bob := Client{"alice", "pw-a"}, Client{"bob", "pw-b"}
		one, err := GenerateFor(o, &st, []Client{alice})
		if err != nil {
			t.Fatal(protocol, err)
		}
		again, err := GenerateFor(o, &st, []Client{alice})
		if err != nil || again.Clients[0] != one.Clients[0] || string(again.Server) != string(one.Server) {
			t.Fatalf("%s: regenerating changed the output", protocol)
		}
		// Adding a client, in front of her, leaves alice's profile unchanged.
		two, err := GenerateFor(o, &st, []Client{bob, alice})
		if err != nil || two.Clients[1] != one.Clients[0] {
			t.Fatalf("%s: adding bob changed alice's profile:\n%s\n%s", protocol, one.Clients[0], two.Clients[1])
		}
		if two.Clients[0] == two.Clients[1] {
			t.Fatalf("%s: two clients share a profile", protocol)
		}
		// A new password gives new credentials.
		changed, err := GenerateFor(o, &st, []Client{{"alice", "new"}})
		if err != nil || changed.Clients[0] == one.Clients[0] {
			t.Fatalf("%s: a password change kept the old credentials", protocol)
		}
		// Another core (new state) never shares credentials.
		other, _ := NewState()
		elsewhere, err := GenerateFor(o, &other, []Client{alice})
		if err != nil || elsewhere.Clients[0] == one.Clients[0] {
			t.Fatalf("%s: two cores issued the same credentials", protocol)
		}
	}
}

func TestWireGuardAddressesNeverMove(t *testing.T) {
	st, _ := NewState()
	if err := st.assignWGAddrs([]Client{{"b", ""}, {"c", ""}}); err != nil {
		t.Fatal(err)
	}
	b, c := st.WGAddrs["b"], st.WGAddrs["c"]
	if b == c || b == "" {
		t.Fatalf("bad assignment %v", st.WGAddrs)
	}
	// "a" sorts first but must not take b's or c's address.
	if err := st.assignWGAddrs([]Client{{"a", ""}, {"b", ""}, {"c", ""}}); err != nil {
		t.Fatal(err)
	}
	if st.WGAddrs["b"] != b || st.WGAddrs["c"] != c || st.WGAddrs["a"] == b || st.WGAddrs["a"] == c {
		t.Fatalf("addresses moved: %v", st.WGAddrs)
	}
	// A removed client frees its address and is forgotten.
	if err := st.assignWGAddrs([]Client{{"c", ""}}); err != nil {
		t.Fatal(err)
	}
	if len(st.WGAddrs) != 1 || st.WGAddrs["c"] != c {
		t.Fatalf("after removal: %v", st.WGAddrs)
	}
}

func TestValidateRepairsPastedLinksAndRejectsPlaceholders(t *testing.T) {
	o := Options{Protocol: "reality", Host: "192.0.2.1", Port: 9443,
		SNI:           "[www.microsoft.com](https://www.microsoft.com)",
		RealityTarget: "[www.microsoft.com:443](https://www.microsoft.com:443)"}
	if err := Validate(&o); err != nil {
		t.Fatal(err)
	}
	if o.SNI != "www.microsoft.com" || o.RealityTarget != "www.microsoft.com:443" {
		t.Fatalf("not repaired: %q %q", o.SNI, o.RealityTarget)
	}
	for _, host := range []string{"SERVER_IP", "your.server ip", "[x](y"} {
		bad := Options{Protocol: "wireguard", Host: host, Port: 51820}
		if err := Validate(&bad); err == nil {
			t.Fatalf("host %q accepted", host)
		}
	}
	bad := Options{Protocol: "reality", Host: "203.0.113.7", Port: 9443, SNI: "a b", RealityTarget: "www.example.com:443"}
	if Validate(&bad) == nil {
		t.Fatal("bad server name accepted")
	}
	bad = Options{Protocol: "reality", Host: "203.0.113.7", Port: 9443, RealityTarget: "www.example.com:99999"}
	if Validate(&bad) == nil {
		t.Fatal("bad target port accepted")
	}
}
