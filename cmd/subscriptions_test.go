package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

func TestSubscriptionRefreshCommandKeepsStoredIdentityAndPrintsNoSecrets(t *testing.T) {
	oldRoot, oldSubs, oldUsage := coreinstall.Root, transport.SubscriptionDir, usage.Dir
	t.Cleanup(func() { coreinstall.Root, transport.SubscriptionDir, usage.Dir = oldRoot, oldSubs, oldUsage })
	coreinstall.Root, transport.SubscriptionDir, usage.Dir = t.TempDir(), filepath.Join(t.TempDir(), "subscriptions"), t.TempDir()
	cfg := &config.Config{PublicAddress: "192.0.2.1", ServerID: strings.Repeat("ab", 16), Backends: config.DefaultBackends(),
		Users:   []config.UserConfig{{Username: "alice", Password: "fixture-private-password", SubToken: strings.Repeat("12", 16), AccountID: strings.Repeat("cd", 16), PeriodID: strings.Repeat("ef", 16)}},
		Tunnels: []config.TunnelConfig{{Tag: "naive", Transport: config.TransportNaive, Backend: config.BackendSOCKS, Enabled: true, Domain: "example.test", Naive: &config.NaiveConfig{Port: 443}}}}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for refresh := 0; refresh < 2; refresh++ {
		var output bytes.Buffer
		command := subscriptionRefreshCommand(func() (*config.Config, error) { return config.LoadFrom(path) })
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs(nil)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "existing_links_kept=1") || strings.Contains(output.String(), cfg.Users[0].SubToken) || strings.Contains(output.String(), cfg.Users[0].Password) {
			t.Fatal("unexpected or private command output")
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("refresh changed account configuration")
		}
		for _, suffix := range []string{".json", ".windows.json", ".status.json", ".windows.status.json"} {
			if _, err := os.Stat(filepath.Join(transport.SubscriptionDir, "sub", cfg.Users[0].SubToken+suffix)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSubscriptionRefreshCommandRefusesMissingConfiguration(t *testing.T) {
	command := subscriptionRefreshCommand(func() (*config.Config, error) { return nil, errors.New("fixture missing config") })
	command.SetArgs(nil)
	command.SilenceUsage = true
	command.SilenceErrors = true
	if command.Execute() == nil {
		t.Fatal("missing configuration was treated as an empty server")
	}
}
