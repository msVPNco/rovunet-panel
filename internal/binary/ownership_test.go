package binary

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledOwnershipRejectsUnsafeAndUnexpectedPaths(t *testing.T) {
	old := ownershipPath
	ownershipPath = filepath.Join(t.TempDir(), "owned.json")
	defer func() { ownershipPath = old }()
	if _, err := ReadInstalledOwnership(); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"schema":2,"files":{}}`, `{"schema":1,"files":{"/etc/passwd":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`, `{"schema":1,"files":{"/usr/local/bin/xray.customer-backup":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`} {
		if err := os.WriteFile(ownershipPath, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadInstalledOwnership(); err == nil {
			t.Fatal("accepted invalid path/schema")
		}
	}
	if err := os.WriteFile(ownershipPath, []byte(`{"schema":1,"files":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ownershipPath, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInstalledOwnership(); err == nil {
		t.Fatal("accepted writable receipt")
	}
}
