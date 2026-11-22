package handlers

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestTrustRefreshFailureRetainsActiveIdentityAndRetiresRevokedCapabilities(t *testing.T) {
	for _, failure := range []string{"missing-certificate", "mixed-private-key-bundle"} {
		t.Run(failure, func(t *testing.T) {
			isolatedSubscriptions(t)
			cfg, certificatePath, certificate := trustFixture25(t)
			cfg.SubscriptionV2 = false
			cfg.Users = append(cfg.Users, subTestConfig().Users[1])
			cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
				Tag: "slipstream-ssh", Transport: config.TransportSlipstream,
				Backend: config.BackendSSH, Domain: "ssh-slip.example.test", Enabled: true,
				Slipstream: &config.SlipstreamConfig{Cert: certificatePath, Key: "DO-NOT-READ-PRIVATE-KEY"},
			})
			ok16(t, WriteSubscriptions(cfg, discardOutput{}))
			active, revoked := cfg.Users[0], cfg.Users[1]
			androidURL, windowsURL := SubscriptionURL(cfg, active), WindowsSubscriptionURL(cfg, active)
			previous := map[string][]byte{}
			for _, suffix := range aliasSuffixes {
				body, err := os.ReadFile(filepath.Join(publicDir(), active.SubToken+suffix))
				ok16(t, err)
				previous[suffix] = body
				if _, err = os.Stat(filepath.Join(publicDir(), revoked.SubToken+suffix)); err != nil {
					t.Fatal("revocation fixture did not publish every account capability", err)
				}
			}

			// A simultaneous feed change must not escape the failed generation.
			// Revoking another account, however, must still retire its old URLs.
			cfg.Users = cfg.Users[:1]
			cfg.Tunnels = append(cfg.Tunnels, config.TunnelConfig{
				Tag: "direct-after-recovery", Transport: config.TransportSSH,
				Backend: config.BackendSSH, Enabled: true,
			})
			switch failure {
			case "missing-certificate":
				ok16(t, os.Remove(certificatePath))
			case "mixed-private-key-bundle":
				mixed := append(append([]byte(nil), certificate...), []byte("-----BEGIN PRIVATE KEY-----\nSYNTHETIC-DO-NOT-PUBLISH\n-----END PRIVATE KEY-----\n")...)
				ok16(t, os.WriteFile(certificatePath, mixed, 0600))
			}
			if err := WriteSubscriptions(cfg, discardOutput{}); err == nil {
				t.Fatal("invalid public certificate was reported as a successful refresh")
			}
			for _, suffix := range aliasSuffixes {
				body, err := os.ReadFile(filepath.Join(publicDir(), active.SubToken+suffix))
				ok16(t, err)
				if !bytes.Equal(body, previous[suffix]) {
					t.Fatalf("failed certificate refresh changed the active account's %s", suffix)
				}
				if _, err = os.Stat(filepath.Join(publicDir(), revoked.SubToken+suffix)); !os.IsNotExist(err) {
					t.Fatalf("revoked account retained its %s capability during refresh failure: %v", suffix, err)
				}
			}

			// Restoring the same certificate resumes publication without changing
			// the account, saved URLs or the previously published certificate pin.
			ok16(t, os.WriteFile(certificatePath, certificate, 0600))
			ok16(t, WriteSubscriptions(cfg, discardOutput{}))
			trust, err := os.ReadFile(filepath.Join(publicDir(), active.SubToken+trustSuffix))
			ok16(t, err)
			if !bytes.Equal(trust, previous[trustSuffix]) {
				t.Fatal("recovery changed the previously valid public certificate identity")
			}
			for _, client := range []string{"android", "windows"} {
				found := false
				for _, item := range savedSubscription(t, active, client).Items {
					found = found || item.ID == "direct-after-recovery"
				}
				if !found {
					t.Fatalf("%s feed did not resume publication after certificate recovery", client)
				}
			}
			for _, suffix := range aliasSuffixes {
				if _, err = os.Stat(filepath.Join(publicDir(), revoked.SubToken+suffix)); !os.IsNotExist(err) {
					t.Fatalf("certificate recovery restored a revoked %s capability: %v", suffix, err)
				}
			}
			if _, err = os.Stat(filepath.Join(publicDir(), active.SubToken+".windows.trust.json")); !os.IsNotExist(err) {
				t.Fatal("recovery published an unsupported Windows-specific trust alias", err)
			}
			if cfg.Users[0] != active || SubscriptionURL(cfg, active) != androidURL || WindowsSubscriptionURL(cfg, active) != windowsURL || cfg.SubscriptionV2 {
				t.Fatal("certificate recovery changed account identity, saved URLs or the v2 opt-in")
			}
		})
	}
}
