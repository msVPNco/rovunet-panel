package coreinstall

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

func TestShadowsocksVariantsRetainHealthyKeysAndRevokeExpiredUsers(t *testing.T) {
	oldRoot, oldUsage, oldStats, oldEpoch := Root, usage.Dir, runStats, runEpoch
	t.Cleanup(func() { Root, usage.Dir, runStats, runEpoch = oldRoot, oldUsage, oldStats, oldEpoch })
	usage.Dir = t.TempDir()
	runStats = func(string, bool) ([]byte, error) { return nil, errors.New("no runtime in policy test") }
	runEpoch = func(string) (string, error) { return "policy-test", nil }
	for _, protocol := range []string{"ss2022-aes128", "ss-aead-chacha20"} {
		t.Run(protocol, func(t *testing.T) {
			Root = t.TempDir()
			seams := fakeSeams(t)
			alice := config.UserConfig{Username: "alice", Password: "fixture-a"}
			bob := config.UserConfig{Username: "bob", Password: "fixture-b"}
			limited := config.UserConfig{Username: "limited", Password: "fixture-c", QuotaGB: 1}
			newUserCore(t, protocol, coreplan.Options{Protocol: protocol, Host: "example.test", Port: 51820}, []config.UserConfig{alice, bob, limited})
			dir := filepath.Join(Root, protocol)
			if Proto(protocol) != "tcp,udp" || !liveUserProtocol(protocol) {
				t.Fatal("runtime compatibility protocol was not registered")
			}
			if _, err := os.Stat(filepath.Join(dir, "clients", "limited.txt")); !os.IsNotExist(err) {
				t.Fatal("traffic-limited user was provisioned")
			}
			before, err := os.ReadFile(filepath.Join(dir, "clients", "bob.txt"))
			if err != nil {
				t.Fatal(err)
			}
			alice.ExpiresAt = "2000-01-01T00:00:00Z"
			if err := syncCore(protocol, []config.UserConfig{alice, bob, limited}); err != nil {
				t.Fatal(err)
			}
			if seams.restarts != 1 || seams.stops != 0 {
				t.Fatal("expiry did not apply a validated restart")
			}
			if _, err := os.Stat(filepath.Join(dir, "clients", "alice.txt")); !os.IsNotExist(err) {
				t.Fatal("expired account retained its client file")
			}
			after, err := os.ReadFile(filepath.Join(dir, "clients", "bob.txt"))
			if err != nil || string(before) != string(after) {
				t.Fatal("expiring alice changed bob's credentials")
			}
			profiles, err := Profiles(bob)
			if err != nil || len(profiles) != 1 || profiles[0].Link != strings.TrimSpace(string(before)) {
				t.Fatal("healthy account export changed")
			}
			bob.Password = "rotated-fixture-b"
			if err := syncCore(protocol, []config.UserConfig{alice, bob, limited}); err != nil {
				t.Fatal(err)
			}
			rotated, err := os.ReadFile(filepath.Join(dir, "clients", "bob.txt"))
			if err != nil || string(rotated) == string(before) || seams.restarts != 2 {
				t.Fatal("password rotation did not revoke the old credential")
			}
			if err := syncCore(protocol, []config.UserConfig{alice, limited}); err != nil {
				t.Fatal(err)
			}
			if seams.stops != 1 || len(clientNames(dir)) != 0 {
				t.Fatal("empty compatibility core was not stopped")
			}
		})
	}
}

func TestShadowsocksVariantsRejectUnreviewedRuntime(t *testing.T) {
	oldXray := XrayPath
	t.Cleanup(func() { XrayPath = oldXray })
	XrayPath = filepath.Join(t.TempDir(), "xray")
	bad := []byte("unreviewed executable must not run")
	for _, path := range []string{XrayPath, dedicatedXrayPath()} {
		if err := os.WriteFile(path, bad, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, protocol := range []string{"ss2022-aes128", "ss-aead-chacha20"} {
		for _, dedicated := range []bool{false, true} {
			if err := ensureOptionsEngine(coreplan.Options{Protocol: protocol, DedicatedXray: dedicated}); err == nil {
				t.Fatalf("%s accepted an unreviewed runtime, dedicated=%v", protocol, dedicated)
			}
		}
	}
	for _, path := range []string{XrayPath, dedicatedXrayPath()} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(bad) {
			t.Fatal("existing unreviewed runtime was overwritten")
		}
	}
}
