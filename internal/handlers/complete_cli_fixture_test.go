package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

// Exports synthetic data only for the private-namespace CLI integration test.
func TestExportCompleteCLIFixture(t *testing.T) {
	dest := os.Getenv("ROVUNET_TEST_COMPLETE_FIXTURE")
	if dest == "" {
		t.Skip("CLI fixture export not requested")
	}
	cert, key := os.Getenv("ROVUNET_TEST_COMPLETE_CERT"), os.Getenv("ROVUNET_TEST_COMPLETE_KEY")
	if cert == "" || key == "" {
		t.Fatal("fixture certificate/key missing")
	}
	cfg, _ := twentyProfileSubscriptionFixture(t)
	list, err := coreinstall.List()
	if err != nil {
		t.Fatal(err)
	}
	options := map[string]coreplan.Options{}
	for _, m := range list {
		data, err := os.ReadFile(filepath.Join(coreinstall.Root, m.Name, "options.json"))
		if err != nil {
			t.Fatal(err)
		}
		var o coreplan.Options
		if err := json.Unmarshal(data, &o); err != nil {
			t.Fatal(err)
		}
		o.Cert, o.Key = cert, key
		options[m.Name] = o
	}
	coreinstall.Root = filepath.Join(dest, "core")
	for name, o := range options {
		subscriptionCoreFixture(t, name, o, cfg.Users[0])
	}
	for i := range cfg.Tunnels {
		tunnel := &cfg.Tunnels[i]
		if tunnel.IsDNSTunnel() {
			tunnel.Port = 5300 + i
		}
		if tunnel.Transport == config.TransportSlipstream {
			tunnel.Slipstream = &config.SlipstreamConfig{Cert: cert, Key: key}
		}
		if tunnel.Transport == config.TransportVayDNS {
			tunnel.VayDNS = &config.VayDNSConfig{PublicKey: strings.Repeat("ab", 32)}
		}
		if v := cfg.Tunnels[i].VLESS; v != nil {
			data, err := os.ReadFile(v.SecretFile)
			if err != nil {
				t.Fatal(err)
			}
			v.SecretFile = filepath.Join(dest, "vless.secret")
			if err := os.WriteFile(v.SecretFile, data, 0600); err != nil {
				t.Fatal(err)
			}
			v.Cert, v.Key = cert, key
		}
	}
	if err := cfg.SaveTo(filepath.Join(dest, "config.json")); err != nil {
		t.Fatal(err)
	}
}
