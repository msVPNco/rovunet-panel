package clientcfg

import (
	"encoding/base64"
	"encoding/json"
	"github.com/msVPNco/rovunet-panel/internal/config"
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
		if len(f) != TotalFieldsV33 || f[0] != "33" {
			t.Fatal("invalid v33 metadata")
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

// TestRovunetV33CredentialsSurvivePipeCharacter is the regression test for
// the plain FSOCKSUser/FSOCKSPass/FSSHUser/FSSHPass fields: they're joined
// into the wire string with "|", so a credential containing "|" used to
// desynchronize every field after it. FCredentialsV33 carries the original,
// un-sanitized values losslessly instead.
func TestRovunetV33CredentialsSurvivePipeCharacter(t *testing.T) {
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	tunnel := &config.TunnelConfig{
		Tag: "soar", Transport: config.TransportSoar, Backend: config.BackendSSH,
		Domain: "r.example.com", Soar: &config.SoarConfig{PublicKey: testSoarKey},
	}
	const user = "ali|ce"  // "|" would otherwise land in the field delimiter itself
	const pass = "p|a|s|s" // multiple "|" to catch an off-by-one in field realignment
	uri, err := GeneratePortableURI(tunnel, nil, cfg, URIOptions{Username: user, Password: pass}, "rovunet")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "rovunet://"))
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Split(string(raw), "|")
	if len(f) != TotalFieldsV33 || f[0] != "33" {
		t.Fatalf("expected a well-formed v33 wire string (len %d, version %q), the \"|\" in the credentials desynchronized field parsing", len(f), f[0])
	}

	// The plain copies must no longer contain "|" (they'd otherwise have
	// split the wire string, which the length/version check above already
	// would have caught indirectly, but check directly too).
	for _, idx := range []int{FSOCKSUser, FSOCKSPass, FSSHUser, FSSHPass} {
		if strings.Contains(f[idx], "|") {
			t.Fatalf("field %d still contains \"|\": %q", idx, f[idx])
		}
	}

	// The lossless copy in FCredentialsV33 must have the original values,
	// "|" included — this is what the app's v33 importer actually uses.
	credJSON, err := base64.StdEncoding.DecodeString(f[FCredentialsV33])
	if err != nil {
		t.Fatalf("FCredentialsV33 is not valid base64: %v", err)
	}
	var creds struct {
		SocksUser string `json:"socksUser"`
		SocksPass string `json:"socksPass"`
		SSHUser   string `json:"sshUser"`
		SSHPass   string `json:"sshPass"`
	}
	if err := json.Unmarshal(credJSON, &creds); err != nil {
		t.Fatalf("FCredentialsV33 is not the expected JSON shape: %v", err)
	}
	if creds.SocksUser != user || creds.SocksPass != pass || creds.SSHUser != user || creds.SSHPass != pass {
		t.Fatalf("lossless credentials mismatch: got %+v, want user=%q pass=%q", creds, user, pass)
	}
}
