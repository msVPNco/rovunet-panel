package clientcfg

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

func displayFields(t *testing.T, link string) []string {
	t.Helper()
	_, payload, ok := strings.Cut(link, "://")
	if !ok {
		t.Fatal("missing native scheme")
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal("invalid native profile")
	}
	return strings.Split(string(raw), "|")
}

func assertOnlyDisplayNameChanged(t *testing.T, before, after string, expected string) {
	t.Helper()
	a, b := displayFields(t, before), displayFields(t, after)
	if len(a) != len(b) || b[FName] != expected || a[FName] == b[FName] {
		t.Fatal("display name was not updated as requested")
	}
	for i := range a {
		if i != FName && a[i] != b[i] {
			t.Fatalf("connection/group/credential field %d changed", i)
		}
	}
}

func TestNativeDisplayNamePreservesEveryOtherField(t *testing.T) {
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	for _, transport := range []string{config.TransportDNSTT, config.TransportNaive, config.TransportSSH, config.TransportSoar} {
		t.Run(transport, func(t *testing.T) {
			tunnel := &config.TunnelConfig{Tag: "fixture", Transport: transport, Backend: config.BackendSSH, Domain: "example.test"}
			options := URIOptions{Username: "alice", Password: "fixture|private", ClientMode: ClientModeNoizDNS}
			for _, format := range []string{"rovunet", "veyra", "legacy", "auto"} {
				before, err := GeneratePortableURI(tunnel, cfg.GetBackend(config.BackendSSH), cfg, options, format)
				if err != nil {
					t.Fatal(err)
				}
				named := options
				named.DisplayName = "Approved · SSH"
				after, err := GeneratePortableURI(tunnel, cfg.GetBackend(config.BackendSSH), cfg, named, format)
				if err != nil {
					t.Fatal(err)
				}
				assertOnlyDisplayNameChanged(t, before, after, named.DisplayName)
			}
			if tunnel.Tag != "fixture" {
				t.Fatal("internal tag changed")
			}
		})
	}
}

func TestCoreDisplayNamePreservesEmbeddedProfileAndCollection(t *testing.T) {
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	for _, embedded := range []string{"vless://fixture-private@example.test:443?security=reality#reality", "[Interface]\nPrivateKey = fixture-only\n[Peer]\nPublicKey = fixture-only\n"} {
		options := URIOptions{Username: "alice"}
		before, err := GenerateCoreURI("reality", embedded, cfg, options)
		if err != nil {
			t.Fatal(err)
		}
		options.DisplayName = "REALITY 1"
		after, err := GenerateCoreURI("reality", embedded, cfg, options)
		if err != nil {
			t.Fatal(err)
		}
		assertOnlyDisplayNameChanged(t, before, after, "REALITY 1")
	}
	if ProfileDisplayName("customer-custom-core") != "" {
		t.Fatal("custom names must remain configured")
	}
}
