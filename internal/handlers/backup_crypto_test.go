package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/actions"
)

func TestBackupKeyFilesAreNotSharedOrFollowed(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "key")
	link := filepath.Join(d, "link")
	os.WriteFile(p, []byte("a sufficiently long passphrase\n"), 0600)
	if _, err := ReadPrivateKeyFile(p); err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, 0644)
	if _, err := ReadPrivateKeyFile(p); err == nil {
		t.Fatal("shared key accepted")
	}
	os.Chmod(p, 0600)
	os.Symlink(p, link)
	if _, err := ReadPrivateKeyFile(link); err == nil {
		t.Fatal("followed key symlink")
	}
	if _, err := protectBackup("ignored", &actions.Context{Args: map[string]string{"plaintext": "true", "recipient": "age1bad"}}); err == nil {
		t.Fatal("ambiguous plaintext accepted")
	}
}
