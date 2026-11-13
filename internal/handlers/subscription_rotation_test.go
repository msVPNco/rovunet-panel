package handlers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

func TestIndependentRotationRetiresAliasesWithoutChangingAccess(t *testing.T) {
	useLocalPublicationTLS(t)
	d := t.TempDir()
	oldRoot, oldCores := transport.SubscriptionDir, coreinstall.Root
	transport.SubscriptionDir = filepath.Join(d, "subscriptions")
	coreinstall.Root = filepath.Join(d, "cores")
	t.Cleanup(func() { transport.SubscriptionDir = oldRoot; coreinstall.Root = oldCores })
	cfg := subTestConfig()
	path := filepath.Join(d, "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	stale, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	before := cfg.Users[0]
	other := cfg.Users[1]
	server := cfg.ServerID
	itemsBefore, err := userSubscriptionItems(cfg, before)
	if err != nil {
		t.Fatal(err)
	}
	if err := RotateSubscriptionLink(cfg, "alice", discardOutput{}); err != nil {
		t.Fatal(err)
	}
	after := cfg.Users[0]
	newToken := after.SubToken
	after.SubToken = before.SubToken
	if newToken == before.SubToken || !reflect.DeepEqual(after, before) || !reflect.DeepEqual(cfg.Users[1], other) || cfg.ServerID != server {
		t.Fatal("rotation altered credentials, account, period, usage baseline or another user")
	}
	itemsAfter, err := userSubscriptionItems(cfg, cfg.Users[0])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(itemsBefore, itemsAfter) {
		t.Fatal("URL rotation changed previously imported runtime configs")
	}
	// A timer loaded before the rotation must not recreate the retired URL.
	if err := WriteSubscriptions(stale, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".json", ".windows.json", ".status.json", ".windows.status.json"} {
		if _, err := os.Stat(filepath.Join(transport.SubscriptionDir, "sub", before.SubToken+suffix)); !os.IsNotExist(err) {
			t.Fatal("old token alias still exists or was resurrected")
		}
		if _, err := os.Stat(filepath.Join(transport.SubscriptionDir, "sub", newToken+suffix)); err != nil {
			t.Fatal("new alias missing")
		}
	}
}

func TestUnsafeTokenCannotEscapeSubscriptionDirectory(t *testing.T) {
	cfg := subTestConfig()
	cfg.Users[0].SubToken = "../../outside"
	if err := WriteSubscriptions(cfg, discardOutput{}); err == nil {
		t.Fatal("unsafe filename reached subscription writer")
	}
}
