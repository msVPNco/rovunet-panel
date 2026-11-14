package handlers

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicationFailurePreservesOld(t *testing.T) {
	isolatedSubscriptions(t)
	cfg := subTestConfig()
	p := filepath.Join(t.TempDir(), "config.json")
	if e := cfg.SaveTo(p); e != nil {
		t.Fatal(e)
	}
	if e := WriteSubscriptions(cfg, discardOutput{}); e != nil {
		t.Fatal(e)
	}
	old := cfg.Users[0].SubToken
	lock := filepath.Join(transport.SubscriptionDir, ".write.lock")
	if e := os.Remove(lock); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(lock, 0700); e != nil {
		t.Fatal(e)
	}
	if e := RotateSubscriptionLink(cfg, cfg.Users[0].Username, discardOutput{}); e == nil {
		t.Fatal("expected publication failure")
	}
	disk, e := config.LoadFrom(p)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, suffix := range []string{".json", ".windows.json", ".status.json", ".windows.status.json"} {
		if _, e := os.Stat(filepath.Join(transport.SubscriptionDir, "sub", old+suffix)); e == nil {
			count++
		}
	}
	if disk.Users[0].SubToken != old || count != 4 {
		t.Fatalf("prior token still selected=%t; old reachable files=%d/4", disk.Users[0].SubToken == old, count)
	}
}
