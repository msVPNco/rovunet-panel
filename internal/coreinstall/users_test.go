package coreinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

func newUserCore(t *testing.T, name string, o coreplan.Options, users []config.UserConfig) {
	t.Helper()
	st, err := coreplan.NewState()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(Root, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := writeUserCore(dir, o, &st, users); err != nil {
		t.Fatal(err)
	}
}

func TestUserCoreProfilesMatchClientFiles(t *testing.T) {
	Root = t.TempDir()
	alice := config.UserConfig{Username: "alice", Password: "pw-a"}
	bob := config.UserConfig{Username: "bob", Password: "pw-b"}
	for _, p := range []coreplan.Options{
		{Protocol: "reality", Host: "203.0.113.1", SNI: "www.example.com", RealityTarget: "www.example.com:443", Port: 9443},
		{Protocol: "wireguard", Host: "203.0.113.1", Port: 51820},
	} {
		newUserCore(t, p.Protocol, p, []config.UserConfig{alice})
	}
	before, err := Profiles(alice)
	if err != nil || len(before) != 2 {
		t.Fatalf("profiles %v, %v", before, err)
	}

	// Adding bob (as user add does) leaves alice's profiles unchanged...
	for _, name := range []string{"reality", "wireguard"} {
		o, st, ok, err := loadUserCore(name)
		if err != nil || !ok {
			t.Fatal(name, ok, err)
		}
		changed, err := writeUserCore(filepath.Join(Root, name), o, &st, []config.UserConfig{bob, alice})
		if err != nil || !changed {
			t.Fatalf("%s: changed=%v err=%v", name, changed, err)
		}
	}
	after, err := Profiles(alice)
	if err != nil {
		t.Fatal(err)
	}
	for i := range before {
		if after[i].Link != before[i].Link {
			t.Fatalf("%s: adding bob changed alice's profile", before[i].Core)
		}
		// ...and is exactly what the core's own client file holds.
		ext := "txt"
		if before[i].Protocol == "wireguard" {
			ext = "conf"
		}
		file, err := os.ReadFile(filepath.Join(Root, before[i].Core, "clients", "alice."+ext))
		if err != nil || strings.TrimSpace(string(file)) != strings.TrimSpace(after[i].Link) {
			t.Fatalf("%s: client file differs from the printed profile", before[i].Core)
		}
	}
	// Reading profiles never changes the stored WireGuard assignment.
	_, st, _, _ := loadUserCore("wireguard")
	if len(st.WGAddrs) != 2 {
		t.Fatalf("WireGuard assignment changed by a read: %v", st.WGAddrs)
	}

	// Removing bob deletes his file; regenerating with no change reports none.
	o, st, _, _ := loadUserCore("reality")
	if _, err := writeUserCore(filepath.Join(Root, "reality"), o, &st, []config.UserConfig{alice}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(Root, "reality", "clients", "bob.txt")); !os.IsNotExist(err) {
		t.Fatal("removed user's client file left behind")
	}
	if changed, err := writeUserCore(filepath.Join(Root, "reality"), o, &st, []config.UserConfig{alice}); err != nil || changed {
		t.Fatalf("no-op regeneration reported changed=%v err=%v", changed, err)
	}
}

func TestDeviceCoresAreLeftAlone(t *testing.T) {
	Root = t.TempDir()
	o := coreplan.Options{Protocol: "hysteria2", Host: "example.org", Port: 8443, Devices: 1, Cert: "c.pem", Key: "k.pem"}
	b, err := coreplan.Generate(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBundle(filepath.Join(Root, "old"), o, b); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := loadUserCore("old"); ok || err != nil {
		t.Fatalf("device core treated as a user core (ok=%v err=%v)", ok, err)
	}
	if err := syncCore("old", []config.UserConfig{{Username: "alice", Password: "x"}}); err != nil {
		t.Fatal(err)
	}
	if p, err := Profiles(config.UserConfig{Username: "alice"}); err != nil || len(p) != 0 {
		t.Fatalf("device core produced user profiles: %v %v", p, err)
	}
}
