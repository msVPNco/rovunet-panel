package handlers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

func fixtureSetupOps(t *testing.T, cfg *config.Config, installs *[]string, backups *int) completeSetupOps {
	t.Helper()
	return completeSetupOps{
		list: coreinstall.List,
		install: func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
			*installs = append(*installs, name)
			subscriptionCoreFixture(t, name, o, users[0])
			return filepath.Join(coreinstall.Root, name), nil
		},
		portFree: func(int, string) error { return nil },
		cert:     func(string) (string, string, bool) { return "fixture-cert", "fixture-key", true },
		backup:   func(*config.Config) (string, error) { *backups++; return "private-fixture.backup", nil },
		reality: func(o *coreplan.Options, avoid string) error {
			o.SNI = "www.example.test"
			o.RealityTarget = "www.example.test:443"
			return nil
		},
	}
}

func coreFileSnapshot(t *testing.T) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(coreinstall.Root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result[path] = string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAutomaticUpgradeAddsFourAndRetainsOldFilesURLsAndCredentials(t *testing.T) {
	cfg, path := twentyProfileSubscriptionFixture(t)
	u := cfg.Users[0]
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	before := savedSubscription(t, u, "android")
	oldFiles := coreFileSnapshot(t)
	androidURL, windowsURL := SubscriptionURL(cfg, u), WindowsSubscriptionURL(cfg, u)
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	for attempt := 0; attempt < 2; attempt++ {
		if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err != nil {
			t.Fatal(err)
		}
		after := savedSubscription(t, u, "android")
		if len(after.Items) != 24 || len(savedSubscription(t, u, "windows").Items) != 24 {
			t.Fatal("complete counts were not exported")
		}
		links := map[string]string{}
		for _, item := range after.Items {
			links[item.ID] = item.Link
		}
		for _, item := range before.Items {
			if links[item.ID] != item.Link {
				t.Fatalf("old profile %s changed", item.ID)
			}
		}
		for path, data := range oldFiles {
			now, err := os.ReadFile(path)
			if err != nil || string(now) != data {
				t.Fatal("an existing core file changed")
			}
		}
		loaded, err := config.LoadFrom(path)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Users[0] != u || SubscriptionURL(loaded, u) != androidURL || WindowsSubscriptionURL(loaded, u) != windowsURL {
			t.Fatal("an account or URL changed")
		}
	}
	if len(installed) != 4 || backups != 1 {
		t.Fatalf("installs=%v backups=%d; retry must be a no-op", installed, backups)
	}
	for _, name := range []string{"managed-ss2022", "managed-trojan", "managed-vlessenc"} {
		var o coreplan.Options
		data, _ := os.ReadFile(filepath.Join(coreinstall.Root, name, "options.json"))
		if jsonErr := json.Unmarshal(data, &o); jsonErr != nil || !o.DedicatedXray {
			t.Fatal("new core did not retain its dedicated runtime")
		}
	}
}

func TestTimedCompleteAccountUsesEligibleTwentyThreeWithoutFalsePending(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	cfg.Users[0].ExpiresAt = "2099-01-01T00:00:00Z"
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err != nil {
		t.Fatal(err)
	}
	if len(savedSubscription(t, cfg.Users[0], "android").Items) != 23 || len(savedSubscription(t, cfg.Users[0], "windows").Items) != 23 {
		t.Fatal("timed eligibility was replaced with a fabricated full set")
	}
}

func TestFreshCompleteSetupExports24And22(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	selected := map[string]bool{}
	for _, transport := range completeTransports() {
		selected[transport] = true
	}
	tunnels := cfg.Tunnels[:0]
	for _, tunnel := range cfg.Tunnels {
		if selected[tunnel.Transport] {
			tunnels = append(tunnels, tunnel)
		}
	}
	cfg.Tunnels = tunnels
	if err := os.RemoveAll(coreinstall.Root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(coreinstall.Root, 0700); err != nil {
		t.Fatal(err)
	}
	var installed []string
	backups := 0
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, fixtureSetupOps(t, cfg, &installed, &backups)); err != nil {
		t.Fatal(err)
	}
	if len(installed) != 9 || backups != 1 {
		t.Fatalf("fresh setup installed %v, backups=%d", installed, backups)
	}
	if len(savedSubscription(t, cfg.Users[0], "android").Items) != 24 || len(savedSubscription(t, cfg.Users[0], "windows").Items) != 24 {
		t.Fatal("fresh complete setup counts differ")
	}
}

func TestAutomaticSetupPartialFailureResumesWithoutRekeying(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	realInstall := ops.install
	ops.install = func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
		if o.Protocol == "ss2022" {
			return "", errors.New("injected install failure")
		}
		return realInstall(name, o, users)
	}
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err == nil {
		t.Fatal("partial setup reported success")
	}
	oldFiles := coreFileSnapshot(t)
	ops.install = realInstall
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err != nil {
		t.Fatal(err)
	}
	for path, data := range oldFiles {
		now, err := os.ReadFile(path)
		if err != nil || string(now) != data {
			t.Fatal("retry changed an already-installed core")
		}
	}
	if len(installed) != 4 {
		t.Fatalf("retry duplicated installs: %v", installed)
	}
}

func TestAutomaticSetupBackupFailureStopsBeforeCreatingCores(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	ops.backup = func(*config.Config) (string, error) { return "", errors.New("backup validation failed") }
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err == nil || len(installed) != 0 {
		t.Fatal("setup continued without a backup")
	}
}

func TestAutomaticSetupSelectsBothTCPAndUDPFreePort(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	ops.portFree = func(port int, proto string) error {
		if port == 28444 && proto == "tcp,udp" {
			return errors.New("UDP occupied")
		}
		return nil
	}
	realInstall := ops.install
	ops.install = func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
		if o.Protocol == "ss2022" && o.Port != 28445 {
			t.Fatal("did not choose a free TCP+UDP port")
		}
		return realInstall(name, o, users)
	}
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticSetupReportsMissingTLSInsteadOfClaiming24(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	ops.cert = func(string) (string, string, bool) { return "", "", false }
	err := ensureCompleteSetupWith(cfg, discardOutput{}, ops)
	if err == nil || !strings.Contains(err.Error(), "valid certificate") {
		t.Fatal("missing TLS was not reported")
	}
	if len(savedSubscription(t, cfg.Users[0], "android").Items) != 23 {
		t.Fatal("missing Trojan was fabricated in the feed")
	}
}

func TestAutomaticSetupPreservesTrafficRestrictions(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	cfg.Users[0].QuotaGB = 1
	var installed []string
	backups := 0
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, fixtureSetupOps(t, cfg, &installed, &backups)); err != nil {
		t.Fatal(err)
	}
	if len(installed) != 0 || backups != 0 {
		t.Fatal("quota-excluded protocols were provisioned")
	}
}

func TestCompletingPartialBaseKeepsExistingBackend(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	if !completeBaseTunnels(cfg) {
		t.Fatal("the 20-profile baseline was not recognized as complete")
	}
	var kept []config.TunnelConfig
	for _, tunnel := range cfg.Tunnels {
		if tunnel.Transport != config.TransportDNSTT || tunnel.Backend != config.BackendSSH {
			kept = append(kept, tunnel)
		}
	}
	cfg.Tunnels = kept
	before, _ := json.Marshal(cfg)
	missing := missingCompleteBackends(cfg, config.TransportDNSTT)
	after, _ := json.Marshal(cfg)
	if len(missing) != 1 || missing[0] != config.BackendSSH || completeBaseTunnels(cfg) {
		t.Fatal("the missing SSH backend was not identified")
	}
	if string(before) != string(after) {
		t.Fatal("planning completion modified existing keys or accounts")
	}
}

func TestAutomaticSetupDoesNotReenableAdministrativelyStoppedCore(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(coreinstall.Root, "managed-ss2022", "state.json")
	data, _ := os.ReadFile(path)
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	state["disabled"] = true
	data, _ = json.Marshal(state)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err == nil {
		t.Fatal("stopped core was reported as complete")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) || len(installed) != 4 {
		t.Fatal("stopped core was modified or installed again")
	}
}
