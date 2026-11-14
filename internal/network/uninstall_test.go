package network

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallResolverOnlyRemovesExactOwnedOverride(t *testing.T) {
	for _, kind := range []string{"owned", "missing", "custom", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			d := t.TempDir()
			p := filepath.Join(d, "rovunet-no-stub.conf")
			admin := filepath.Join(d, "administrator-resolv.conf")
			if err := os.WriteFile(admin, []byte("administrator DNS"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "owned":
				err = os.WriteFile(p, []byte("[Resolve]\nDNSStubListener=no\n"), 0644)
			case "custom":
				err = os.WriteFile(p, []byte("[Resolve]\nDNS=192.0.2.53\n"), 0644)
			case "symlink":
				err = os.Symlink(admin, p)
			}
			if err != nil {
				t.Fatal(err)
			}
			removed, err := removeResolvedStubDropin(p)
			if removed != (kind == "owned") || (err != nil) != (kind == "custom" || kind == "symlink") {
				t.Fatalf("unexpected result: removed=%t err=%v", removed, err)
			}
			b, err := os.ReadFile(admin)
			if err != nil || string(b) != "administrator DNS" {
				t.Fatal("administrator DNS changed")
			}
			if kind == "owned" {
				if _, err := os.Lstat(p); !os.IsNotExist(err) {
					t.Fatal("owned dropin remains")
				}
			}
		})
	}
}
