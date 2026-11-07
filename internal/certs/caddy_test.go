package certs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCaddy(t *testing.T) {
	base := t.TempDir()
	old := CaddyDataDirs
	CaddyDataDirs = []string{base}
	defer func() { CaddyDataDirs = old }()

	if _, _, ok := FindCaddy("example.org"); ok {
		t.Fatal("found a certificate that does not exist")
	}
	dir := filepath.Join(base, "certificates", "acme-v02.api.letsencrypt.org-directory", "example.org")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.org.crt"), []byte("c"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := FindCaddy("example.org"); ok {
		t.Fatal("a certificate without its key was reported")
	}
	if err := os.WriteFile(filepath.Join(dir, "example.org.key"), []byte("k"), 0600); err != nil {
		t.Fatal(err)
	}
	crt, key, ok := FindCaddy("example.org")
	if !ok || filepath.Base(crt) != "example.org.crt" || filepath.Base(key) != "example.org.key" {
		t.Fatalf("got %q %q %v", crt, key, ok)
	}
	for _, bad := range []string{"", "../example.org", "a/b"} {
		if _, _, ok := FindCaddy(bad); ok {
			t.Fatalf("%q accepted", bad)
		}
	}
}
