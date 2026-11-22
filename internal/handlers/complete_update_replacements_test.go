package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

func replacementFeeds(t *testing.T, cfg *config.Config) map[string]subFeed {
	t.Helper()
	return map[string]subFeed{
		"android": savedSubscription(t, cfg.Users[0], "android"),
		"windows": savedSubscription(t, cfg.Users[0], "windows"),
	}
}

func assertAppliedReplacementFeeds(t *testing.T, cfg *config.Config, before map[string]subFeed, replaced map[string]string) {
	t.Helper()
	for client, old := range before {
		after := savedSubscription(t, cfg.Users[0], client)
		links := map[string]string{}
		for _, item := range after.Items {
			links[item.ID] = item.Link
		}
		if len(after.Items) != len(old.Items) {
			t.Fatal("replacement created unrelated preset profiles or omitted old items")
		}
		for _, item := range old.Items {
			if target := replaced[item.ID]; target != "" {
				if links[item.ID] != "" || links[target] == "" {
					t.Fatalf("%s feed still advertises retired %s or omits its replacement", client, item.ID)
				}
			} else if links[item.ID] != item.Link {
				t.Fatalf("%s feed changed unrelated item %s", client, item.ID)
			}
		}
	}
}

func TestCoherentPartialUpdateReplacesOnlyExistingDefaultsUnderSharedLock(t *testing.T) {
	for _, secondFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "second-replacement-fails"}[secondFails], func(t *testing.T) {
			cfg, h := replacementTestFixture(t)
			for i, spec := range completeReplacements {
				replacementAddOld(t, cfg, h, spec, []int{51822, 33446}[i])
			}
			// A real, bound unlimited ledger makes this an existing coherent
			// installation. Its partial Naive-only transport selection is kept.
			binding, err := installPeriod(cfg.Path(), cfg.ServerID, cfg.Users[0], 4096)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Users[0].SQLiteMeter = &binding
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			if !requiresCoherent(cfg) || completeBaseTunnels(cfg) {
				t.Fatal("fixture is not a coherent partial installation")
			}
			subscriptionCoreFixture(t, "managed-ss2022", coreplan.Options{Protocol: "ss2022", Host: cfg.PublicAddress, Port: 28445}, cfg.Users[0])
			unrelated := replacementSnapshot(t, filepath.Join(coreinstall.Root, "managed-ss2022"))
			if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
				t.Fatal(err)
			}
			before := replacementFeeds(t, cfg)
			configBefore, _ := os.ReadFile(cfg.Path())
			android, windows := SubscriptionURL(cfg, cfg.Users[0]), WindowsSubscriptionURL(cfg, cfg.Users[0])
			amountBefore, _, err := boundAmount(cfg.ServerID, cfg.Users[0])
			if err != nil || amountBefore.Confirmed != 4096 {
				t.Fatal("fixture usage opening is unavailable")
			}
			install := h.ops.install
			if secondFails {
				h.ops.install = func(name string, o coreplan.Options, users []config.UserConfig) (string, error) {
					if name == completeReplacements[1].newName {
						return "", errors.New("fixture second replacement failure")
					}
					return install(name, o, users)
				}
			}
			previous := replaceSetupDefaults
			t.Cleanup(func() { replaceSetupDefaults = previous })
			lockChecked := false
			replaceSetupDefaults = func(c *config.Config, out actions.OutputWriter) (map[string]bool, error) {
				file, err := os.OpenFile(filepath.Join(filepath.Dir(coreinstall.Root), ".complete-setup.lock"), os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
					t.Fatal("partial update did not retain the shared complete-setup lock")
				}
				lockChecked = true
				return migrateCompleteReplacementsWith(c, out, h.ops)
			}
			err = completeCoherentCoreSetup(cfg, discardOutput{})
			if (err != nil) != secondFails || !lockChecked {
				t.Fatalf("partial update result: %v", err)
			}
			replaced := map[string]string{"wireguard": "managed-ss2022-aes128"}
			if !secondFails {
				replaced["managed-tuic"] = "managed-ss-aead-chacha20"
			}
			assertAppliedReplacementFeeds(t, cfg, before, replaced)
			first := replacementSnapshot(t, filepath.Join(coreinstall.Root, completeReplacements[0].newName))
			h.ops.install = install
			if err := completeCoherentCoreSetup(cfg, discardOutput{}); err != nil {
				t.Fatal(err)
			}
			replaced["managed-tuic"] = "managed-ss-aead-chacha20"
			assertAppliedReplacementFeeds(t, cfg, before, replaced)
			if h.installs != 2 || !reflect.DeepEqual(first, replacementSnapshot(t, filepath.Join(coreinstall.Root, completeReplacements[0].newName))) || !reflect.DeepEqual(unrelated, replacementSnapshot(t, filepath.Join(coreinstall.Root, "managed-ss2022"))) {
				t.Fatal("retry recreated a ready replacement or changed an unrelated core")
			}
			cores, err := coreinstall.List()
			if err != nil || len(cores) != 3 {
				t.Fatal("partial update created unrelated preset cores")
			}
			for i, spec := range completeReplacements {
				meta, err := coreinstall.ReadMeta(spec.newName)
				if err != nil || meta.Port != []int{51822, 33446}[i] {
					t.Fatal("partial update reset a saved listener port")
				}
			}
			amountAfter, _, err := boundAmount(cfg.ServerID, cfg.Users[0])
			configAfter, _ := os.ReadFile(cfg.Path())
			if err != nil || amountAfter != amountBefore || string(configAfter) != string(configBefore) || android != SubscriptionURL(cfg, cfg.Users[0]) || windows != WindowsSubscriptionURL(cfg, cfg.Users[0]) {
				t.Fatal("partial update changed account usage, configuration or saved URLs")
			}
		})
	}
}

func TestAutomaticSetupPublishesReplacementsBeforeLaterEarlyFailure(t *testing.T) {
	for _, point := range []string{"list", "general-backup"} {
		t.Run(point, func(t *testing.T) {
			cfg, _ := twentyProfileSubscriptionFixture(t)
			subscriptionCoreFixture(t, "managed-tuic", coreplan.Options{Protocol: "tuic", Host: cfg.PublicAddress, Port: 33446, Cert: "cert", Key: "key"}, cfg.Users[0])
			if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
				t.Fatal(err)
			}
			before := replacementFeeds(t, cfg)
			android, windows := SubscriptionURL(cfg, cfg.Users[0]), WindowsSubscriptionURL(cfg, cfg.Users[0])
			var installs []string
			backups := 0
			ops := fixtureSetupOps(t, cfg, &installs, &backups)
			failure := errors.New("fixture post-replacement prerequisite failure")
			if point == "list" {
				ops.list = func() ([]coreinstall.Meta, error) { return nil, failure }
			} else {
				ops.backup = func(*config.Config) (string, error) { return "", failure }
			}
			if err := ensureCompleteSetupWith(cfg, discardOutput{}, ops); !errors.Is(err, failure) {
				t.Fatalf("later prerequisite failure was not returned: %v", err)
			}
			if len(installs) != 2 {
				t.Fatal("fixture did not complete exactly the two replacements before failure")
			}
			assertAppliedReplacementFeeds(t, cfg, before, map[string]string{
				"wireguard": "managed-ss2022-aes128", "managed-tuic": "managed-ss-aead-chacha20",
			})
			if android != SubscriptionURL(cfg, cfg.Users[0]) || windows != WindowsSubscriptionURL(cfg, cfg.Users[0]) {
				t.Fatal("post-replacement failure changed saved subscription URLs")
			}
		})
	}
}
