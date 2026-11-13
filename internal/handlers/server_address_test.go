package handlers

import (
	"path/filepath"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestSetPublicAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Default().SaveTo(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := setPublicAddress(cfg, "203.0.113.7:22"); err == nil || cfg.PublicAddress != "" {
		t.Errorf("address with a port accepted: %v, %q", err, cfg.PublicAddress)
	}
	if err := setPublicAddress(cfg, "203.0.113.7"); err != nil {
		t.Fatal(err)
	}
	if saved, _ := config.LoadFrom(path); saved.PublicAddress != "203.0.113.7" {
		t.Errorf("saved %q", saved.PublicAddress)
	}
	if err := setPublicAddress(cfg, "auto"); err != nil {
		t.Fatal(err)
	}
	if saved, _ := config.LoadFrom(path); saved.PublicAddress != "" {
		t.Errorf(`"auto" left %q`, saved.PublicAddress)
	}
}

// TestEnsureServerAddressQuiet: nothing to ask or warn about when no tunnel
// connects by address, or when the public address is known.
func TestEnsureServerAddressQuiet(t *testing.T) {
	dnsOnly := []config.TunnelConfig{{Tag: "dnstt", Transport: config.TransportDNSTT, Domain: "t.example.com"}}
	direct := []config.TunnelConfig{{Tag: "ssh", Transport: config.TransportSSH}}

	out := &printOutput{}
	ensureServerAddress(&config.Config{}, out, dnsOnly)
	ensureServerAddress(&config.Config{PublicAddress: "203.0.113.7"}, out, direct)
	if len(out.lines) != 0 {
		t.Errorf("printed %q", out.lines)
	}
}

func TestSetPublicAddressWithExtras(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Default().SaveTo(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := setPublicAddress(cfg, "auto,198.51.100.9, 198.51.100.9 ,198.51.100.10"); err != nil {
		t.Fatal(err)
	}
	if cfg.PublicAddress != "" || len(cfg.ExtraAddresses) != 2 {
		t.Fatalf("got %q %v", cfg.PublicAddress, cfg.ExtraAddresses)
	}
	if got := addressList(cfg); got != "auto,198.51.100.9,198.51.100.10" {
		t.Fatalf("list %q", got)
	}
	if err := setPublicAddress(cfg, "auto,198.51.100.10,198.51.100.9"); err != nil {
		t.Fatal(err)
	}
	if saved, err := config.LoadFrom(path); err != nil || addressList(saved) != "auto,198.51.100.9,198.51.100.10" {
		t.Fatalf("reordering aliases moved saved subscription IDs: %v", err)
	}
	if saved, _ := config.LoadFrom(path); len(saved.ExtraAddresses) != 2 {
		t.Fatalf("saved %v", saved.ExtraAddresses)
	}
	if err := setPublicAddress(cfg, "198.51.100.9,bad address"); err == nil || len(cfg.ExtraAddresses) != 2 {
		t.Fatalf("invalid extra accepted: %v %v", err, cfg.ExtraAddresses)
	}
	if err := setPublicAddress(cfg, "auto"); err != nil || len(cfg.ExtraAddresses) != 0 {
		t.Fatalf("auto should clear extras: %v %v", err, cfg.ExtraAddresses)
	}
}
