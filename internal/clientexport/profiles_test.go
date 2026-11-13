package clientexport

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

func TestAccountPolicyAndEnabledStateApplyToEveryExportFormat(t *testing.T) {
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	tunnels := []config.TunnelConfig{
		{Tag: "soar", Enabled: true, Transport: config.TransportSoar, Backend: config.BackendDirect, Domain: "r.example.test", Soar: &config.SoarConfig{PublicKey: strings.Repeat("ab", 32)}},
		{Tag: "ssh", Enabled: true, Transport: config.TransportSSH, Backend: config.BackendSSH},
		{Tag: "disabled", Transport: config.TransportSSH, Backend: config.BackendSSH},
	}
	for _, format := range []string{"auto", "rovunet", "legacy"} {
		for _, tc := range []struct {
			user config.UserConfig
			want int
		}{
			{config.UserConfig{Username: "alice", Password: "fixture"}, 2},
			{config.UserConfig{Username: "alice", Password: "fixture", ExpiresAt: "2099-01-01T00:00:00Z"}, 1},
			{config.UserConfig{Username: "alice", Password: "fixture", QuotaGB: 5}, 0},
		} {
			got, err := Build(cfg, tc.user, tunnels, Options{Format: format})
			if err != nil || len(got) != tc.want {
				t.Fatalf("format %s: profiles=%d want=%d err=%v", format, len(got), tc.want, err)
			}
		}
	}
}

func TestReleasedClientCapabilitiesAndAccountPolicy(t *testing.T) {
	old := coreinstall.Root
	coreinstall.Root = t.TempDir()
	t.Cleanup(func() { coreinstall.Root = old })
	u := config.UserConfig{Username: "alice", Password: "fixture"}
	for _, protocol := range []string{"amneziawg", "ss2022", "trojan", "vlessenc"} {
		o := coreplan.Options{Protocol: protocol, Host: "192.0.2.1", SNI: "example.test", Port: 28444, Cert: "cert", Key: "key", Subnet: "10.203.72.0/24", Egress: "eth0"}
		st, err := coreplan.NewState()
		if err != nil {
			t.Fatal(err)
		}
		b, err := coreplan.GenerateFor(o, &st, []coreplan.Client{{Name: u.Username, Secret: u.Password}})
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(coreinstall.Root, protocol)
		if err := coreinstall.WriteBundle(dir, o, b); err != nil {
			t.Fatal(err)
		}
		for file, value := range map[string]any{"options.json": o, "state.json": st} {
			data, _ := json.Marshal(value)
			if err := os.WriteFile(filepath.Join(dir, file), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Rename(filepath.Join(dir, "clients", "device-001."+b.Extension), filepath.Join(dir, "clients", u.Username+"."+b.Extension)); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{PublicAddress: "192.0.2.1", Backends: config.DefaultBackends()}
	for _, tc := range []struct {
		platform    string
		quota, want int
	}{{"android", 0, 4}, {"windows", 0, 4}, {"android", 1, 0}, {"windows", 1, 0}} {
		u.QuotaGB = tc.quota
		got, err := Build(cfg, u, nil, Options{Client: tc.platform, StatsWorking: func(string) bool { return true }})
		if err != nil || len(got) != tc.want {
			t.Fatalf("platform=%s quota=%d profiles=%d want=%d err=%v", tc.platform, tc.quota, len(got), tc.want, err)
		}
	}
}
