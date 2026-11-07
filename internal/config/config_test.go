package config

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestSaveToWritesOwnerOnlyPermissions covers config.json's permission bits:
// the file holds plaintext user/tunnel credentials, so it must never be
// group- or world-readable, whether it's being created for the first time
// or re-saved by a build newer than the one that first wrote it.
func TestSaveToWritesOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits don't apply on windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	c := &Config{}
	if err := c.SaveTo(path); err != nil {
		t.Fatalf("SaveTo (new file): %v", err)
	}
	assertOwnerOnly(t, path)

	// Simulate an existing config.json left world-readable by a build from
	// before this fix, then save again as a newer build would on upgrade.
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatalf("chmod to simulate pre-fix file: %v", err)
	}
	if err := c.SaveTo(path); err != nil {
		t.Fatalf("SaveTo (pre-existing file): %v", err)
	}
	assertOwnerOnly(t, path)
}

func assertOwnerOnly(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("%s has mode %04o, want 0600 (must not be group/world-readable)", path, got)
	}
}

// TestServerHostnames: ns.<parent> for DNS tunnels and the domain for
// NaiveProxy, once each; tunnels without a domain have none.
func TestServerHostnames(t *testing.T) {
	got := importable().ServerHostnames()
	want := []string{"ns.example.com", "example.com"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	c := &Config{Tunnels: []TunnelConfig{
		{Transport: TransportDNSTT, Domain: "t.a.example.com"},
		{Transport: TransportNaive, Domain: "n.example.org"},
		{Transport: TransportSoar, Domain: "s.a.example.com"},
		{Transport: TransportStunTLS},
		{Transport: TransportSSH},
	}}
	if got, want := c.ServerHostnames(), []string{"ns.a.example.com", "n.example.org"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParentDomain(t *testing.T) {
	for in, want := range map[string]string{
		"t.example.com": "example.com", "a.b.example.com": "b.example.com",
		"example.com": "example.com", "t..example.com": "example.com",
	} {
		if got := ParentDomain(in); got != want {
			t.Errorf("ParentDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidatePublicAddress(t *testing.T) {
	for _, ok := range []string{"203.0.113.7", "2001:db8::1", "vpn.example.com"} {
		if err := ValidatePublicAddress(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "203.0.113.7:22", "https://vpn.example.com", "vpn example.com", "localhost", "-x"} {
		if ValidatePublicAddress(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
