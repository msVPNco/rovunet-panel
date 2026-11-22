package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

func fixtureSetupOps(t *testing.T, cfg *config.Config, installs *[]string, backups *int) completeSetupOps {
	t.Helper()
	ops := completeSetupOps{
		list: coreinstall.List,
		install: func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
			if err := coreplan.Validate(&o); err != nil {
				return "", err
			}
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
	replacement := fixtureSetupReplacementOps(t, ops.install)
	ops.replaceDefaults = func(cfg *config.Config, out actions.OutputWriter) (map[string]bool, error) {
		return migrateCompleteReplacementsWith(cfg, out, replacement)
	}
	return ops
}

// Exercise the real replacement orchestration and private backup with synthetic
// core files. Runtime execution belongs to test-complete-setup.py's namespaces.
func fixtureSetupReplacementOps(t *testing.T, install func(string, coreplan.Options, []config.UserConfig) (string, error)) replacementOps {
	t.Helper()
	setDisabled := func(name string, disabled bool) error {
		_, st, err := replacementOptions(name)
		if err != nil {
			return err
		}
		st.Disabled = disabled
		data, err := json.Marshal(st)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(coreinstall.Root, name, "state.json"), data, 0600)
	}
	return replacementOps{
		stop:  func(name string) error { return setDisabled(name, true) },
		start: func(name string) error { return setDisabled(name, false) },
		status: func(name string) (string, error) {
			_, st, err := replacementOptions(name)
			if err != nil {
				return "", err
			}
			if st.Disabled || st.Paused {
				return "inactive", nil
			}
			return "active", nil
		},
		confirm: func(name string) error {
			_, st, err := replacementOptions(name)
			if err != nil {
				return err
			}
			if st.Disabled || st.Paused {
				return fmt.Errorf("fixture core %s is stopped", name)
			}
			_, err = os.Stat(filepath.Join(coreinstall.Root, name, "applied.json"))
			return err
		},
		remove:  func(name string) error { return os.RemoveAll(filepath.Join(coreinstall.Root, name)) },
		install: install,
		backup:  backupReplacementCore,
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

func assertCompleteShadowsocks(t *testing.T, name, protocol, method string, port int) {
	t.Helper()
	o, st, err := replacementOptions(name)
	if err != nil {
		t.Fatal(err)
	}
	m, err := coreinstall.ReadMeta(name)
	if err != nil || m.Protocol != protocol || m.Port != port || m.Proto != "tcp,udp" ||
		o.Protocol != protocol || o.Port != port || !o.DedicatedXray || st.Disabled || st.Paused {
		t.Fatalf("%s lost its alias, dedicated runtime, saved port, or TCP/UDP support: %+v, %+v, %v", name, m, o, err)
	}
	data, err := os.ReadFile(filepath.Join(coreinstall.Root, name, "server.json"))
	if err != nil {
		t.Fatal(err)
	}
	var server struct {
		Inbounds []struct {
			Protocol string
			Port     int
			Settings struct {
				Method, Network string
				Clients         []struct{ Method string }
			}
		}
	}
	if err := json.Unmarshal(data, &server); err != nil || len(server.Inbounds) != 1 {
		t.Fatal("replacement server config is missing its single inbound")
	}
	in := server.Inbounds[0]
	if in.Protocol != "shadowsocks" || in.Port != port || in.Settings.Network != "tcp,udp" || len(in.Settings.Clients) != 1 {
		t.Fatal("replacement server config does not match the saved listener or account")
	}
	if protocol == "ss-aead-chacha20" {
		if in.Settings.Method != "" || in.Settings.Clients[0].Method != method {
			t.Fatal("classic ChaCha20 was replaced by a different cipher")
		}
	} else if in.Settings.Method != method {
		t.Fatal("SS2022 cipher differs from the requested variant")
	}
}

func assertCompleteReplacementBackup(t *testing.T, oldName, newName, protocol string, port int, oldFiles map[string]string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(coreinstall.Root, oldName)); !os.IsNotExist(err) {
		t.Fatalf("retired default %s remains on disk: %v", oldName, err)
	}
	journal := filepath.Join(filepath.Dir(coreinstall.Root), "core-replacements", oldName+".json")
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	var record replacementRecord
	if err := json.Unmarshal(data, &record); err != nil || record.Stage != "done" || record.OldName != oldName ||
		record.NewName != newName || record.NewProtocol != protocol || record.Port != port || !record.WasActive {
		t.Fatalf("incomplete replacement record for %s: %+v, %v", oldName, record, err)
	}
	if info, err := os.Stat(journal); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("replacement journal is not private")
	}
	if info, err := os.Stat(record.Backup); err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("replacement backup directory is not private")
	}
	prefix := filepath.Join(coreinstall.Root, oldName) + string(filepath.Separator)
	for path, before := range oldFiles {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		backupPath := filepath.Join(record.Backup, strings.TrimPrefix(path, prefix))
		after, err := os.ReadFile(backupPath)
		if err != nil || string(after) != before {
			t.Fatalf("replacement backup changed original %s", path)
		}
		if info, err := os.Stat(backupPath); err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("replacement backup member is not private: %s", backupPath)
		}
	}
}

func TestAutomaticUpgradeReplacesDefaultsAndRetainsUnrelatedFilesURLsAndCredentials(t *testing.T) {
	for _, withTUIC := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-tuic-%v", withTUIC), func(t *testing.T) {
			testAutomaticUpgradeReplacements(t, withTUIC)
		})
	}
}

func testAutomaticUpgradeReplacements(t *testing.T, withTUIC bool) {
	t.Helper()
	cfg, path := twentyProfileSubscriptionFixture(t)
	u := cfg.Users[0]
	retired := map[string]bool{"wireguard": true}
	chachaPort := 33444
	if withTUIC {
		// Use a saved nondefault port to prove upgrades do not reset listeners.
		chachaPort = 33446
		retired["managed-tuic"] = true
		subscriptionCoreFixture(t, "managed-tuic", coreplan.Options{Protocol: "tuic", Host: cfg.PublicAddress, SNI: "example.test", Port: chachaPort, Cert: "cert", Key: "key"}, u)
	}
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	before := savedSubscription(t, u, "android")
	wantBefore := 20
	if withTUIC {
		wantBefore++
	}
	if len(before.Items) != wantBefore {
		t.Fatalf("baseline profiles=%d, want %d", len(before.Items), wantBefore)
	}
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
		if len(after.Items) != 30 || len(savedSubscription(t, u, "windows").Items) != 30 {
			t.Fatal("complete counts were not exported")
		}
		links := map[string]string{}
		for _, item := range after.Items {
			links[item.ID] = item.Link
		}
		for _, item := range before.Items {
			if retired[item.ID] {
				if _, ok := links[item.ID]; ok {
					t.Fatalf("retired default %s remains in the feed", item.ID)
				}
				continue
			}
			if links[item.ID] != item.Link {
				t.Fatalf("old profile %s changed", item.ID)
			}
		}
		for path, data := range oldFiles {
			rel, err := filepath.Rel(coreinstall.Root, path)
			if err != nil {
				t.Fatal(err)
			}
			if retired[strings.SplitN(rel, string(filepath.Separator), 2)[0]] {
				continue
			}
			now, err := os.ReadFile(path)
			if err != nil || string(now) != data {
				t.Fatal("an existing core file changed")
			}
		}
		for _, name := range []string{"managed-ss2022-aes128", "managed-ss-aead-chacha20"} {
			if links[name] == "" {
				t.Fatalf("missing replacement profile %s", name)
			}
		}
		assertCompleteShadowsocks(t, "managed-ss2022-aes128", "ss2022-aes128", "2022-blake3-aes-128-gcm", 18447)
		assertCompleteShadowsocks(t, "managed-ss-aead-chacha20", "ss-aead-chacha20", "chacha20-ietf-poly1305", chachaPort)
		assertCompleteReplacementBackup(t, "wireguard", "managed-ss2022-aes128", "ss2022-aes128", 18447, oldFiles)
		if withTUIC {
			assertCompleteReplacementBackup(t, "managed-tuic", "managed-ss-aead-chacha20", "ss-aead-chacha20", chachaPort, oldFiles)
		}
		loaded, err := config.LoadFrom(path)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Users[0] != u || SubscriptionURL(loaded, u) != androidURL || WindowsSubscriptionURL(loaded, u) != windowsURL {
			t.Fatal("an account or URL changed")
		}
	}
	if len(installed) != 11 || backups != 1 {
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

func TestTimedCompleteAccountUsesEligibleTwentyNineWithoutFalsePending(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	cfg.Users[0].ExpiresAt = "2099-01-01T00:00:00Z"
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err != nil {
		t.Fatal(err)
	}
	if len(savedSubscription(t, cfg.Users[0], "android").Items) != 29 || len(savedSubscription(t, cfg.Users[0], "windows").Items) != 29 {
		t.Fatal("timed eligibility was replaced with a fabricated full set")
	}
}

func TestFreshCompleteSetupExportsThirtyWithFixedReplacementPorts(t *testing.T) {
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
	if len(installed) != 15 || backups != 1 {
		t.Fatalf("fresh setup installed %v, backups=%d", installed, backups)
	}
	if len(savedSubscription(t, cfg.Users[0], "android").Items) != 30 || len(savedSubscription(t, cfg.Users[0], "windows").Items) != 30 {
		t.Fatal("fresh complete setup counts differ")
	}
	assertCompleteShadowsocks(t, "managed-ss2022-aes128", "ss2022-aes128", "2022-blake3-aes-128-gcm", 51820)
	assertCompleteShadowsocks(t, "managed-ss-aead-chacha20", "ss-aead-chacha20", "chacha20-ietf-poly1305", 33444)
	assertCompleteShadowsocks(t, "managed-ss2022", "ss2022", "2022-blake3-aes-256-gcm", 28444)
	for _, name := range installed {
		if name == "wireguard" || name == "managed-tuic" {
			t.Fatalf("fresh setup installed retired default %s", name)
		}
	}
	m, err := coreinstall.ReadMeta("managed-awg")
	if err != nil || m.Protocol != "amneziawg" || m.Port != 51840 {
		t.Fatal("ordinary WireGuard replacement changed the independent AWG default")
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
	if len(installed) != 11 {
		t.Fatalf("retry duplicated installs: %v", installed)
	}
}

func TestAutomaticSetupBackupFailureStopsBeforeCreatingCores(t *testing.T) {
	for _, withReplacement := range []bool{false, true} {
		t.Run(fmt.Sprintf("replacement-%v", withReplacement), func(t *testing.T) {
			cfg, _ := twentyProfileSubscriptionFixture(t)
			if !withReplacement {
				// Exercise the ordinary setup backup gate independently of the
				// earlier, per-core replacement backup gate.
				if err := os.RemoveAll(filepath.Join(coreinstall.Root, "wireguard")); err != nil {
					t.Fatal(err)
				}
			}
			var installed []string
			backups := 0
			ops := fixtureSetupOps(t, cfg, &installed, &backups)
			ops.backup = func(*config.Config) (string, error) { return "", errors.New("backup validation failed") }
			replacement := fixtureSetupReplacementOps(t, ops.install)
			replacement.backup = func(string, string) error { return errors.New("backup validation failed") }
			ops.replaceDefaults = func(cfg *config.Config, out actions.OutputWriter) (map[string]bool, error) {
				return migrateCompleteReplacementsWith(cfg, out, replacement)
			}
			before := coreFileSnapshot(t)
			if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); err == nil || len(installed) != 0 {
				t.Fatal("setup continued without a backup")
			}
			for path, data := range before {
				if after, err := os.ReadFile(path); err != nil || string(after) != data {
					t.Fatal("backup failure changed an existing core")
				}
			}
		})
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

func TestAutomaticSetupReportsMissingTLSInsteadOfClaimingThirty(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	var installed []string
	backups := 0
	ops := fixtureSetupOps(t, cfg, &installed, &backups)
	ops.cert = func(string) (string, string, bool) { return "", "", false }
	err := ensureCompleteSetupWith(cfg, discardOutput{}, ops)
	if err == nil || !strings.Contains(err.Error(), "valid certificate") {
		t.Fatal("missing TLS was not reported")
	}
	if len(savedSubscription(t, cfg.Users[0], "android").Items) != 27 {
		t.Fatal("missing TLS protocols were fabricated in the feed")
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
	if string(after) != string(data) || len(installed) != 11 {
		t.Fatal("stopped core was modified or installed again")
	}
}

// The CLI test calls this after TestExportCompleteCLIFixture in a separate
// process so both legacy automatic targets exist before capturing its baseline.
func TestExportCompleteReplacementCLIFixture(t *testing.T) {
	dest := os.Getenv("ROVUNET_TEST_COMPLETE_FIXTURE")
	if dest == "" {
		t.Skip("CLI fixture export not requested")
	}
	cert, key := os.Getenv("ROVUNET_TEST_COMPLETE_CERT"), os.Getenv("ROVUNET_TEST_COMPLETE_KEY")
	if cert == "" || key == "" {
		t.Fatal("fixture certificate/key missing")
	}
	cfg, err := config.LoadFrom(filepath.Join(dest, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Users) != 1 {
		t.Fatal("expected one synthetic CLI fixture account")
	}
	oldRoot := coreinstall.Root
	t.Cleanup(func() { coreinstall.Root = oldRoot })
	coreinstall.Root = filepath.Join(dest, "core")
	o := coreplan.Options{Protocol: "tuic", Host: cfg.PublicAddress, Port: 33446, Cert: cert, Key: key}
	if err := coreplan.Validate(&o); err != nil {
		t.Fatal(err)
	}
	subscriptionCoreFixture(t, "managed-tuic", o, cfg.Users[0])
}
