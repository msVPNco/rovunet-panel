package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnsureVLESSSecretKeepsExisting: the secret is created once and then
// reused, so UUIDs survive a tunnel being removed and added again.
func TestEnsureVLESSSecretKeepsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "vless.secret")
	if err := EnsureVLESSSecret(path); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureVLESSSecret(path); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatal("existing secret was replaced")
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0600 {
		t.Errorf("mode %v, want 0600", st.Mode().Perm())
	}
	// a corrupt file is an error, never silently replaced
	os.WriteFile(path, []byte("not hex\n"), 0600)
	if err := EnsureVLESSSecret(path); err == nil {
		t.Error("corrupt secret accepted")
	}
}

// TestVLESSDefaultPath: the same domain always gets the same path; other
// domains or secrets get another one; it is a plain, safe API-like path.
func TestVLESSDefaultPath(t *testing.T) {
	a := []byte(strings.Repeat("a", 32))
	b := []byte(strings.Repeat("b", 32))
	p := VLESSDefaultPath(a, "cdn.example.com")
	if p != VLESSDefaultPath(a, "cdn.example.com") {
		t.Fatal("path not stable")
	}
	if p == VLESSDefaultPath(a, "cdn2.example.com") || p == VLESSDefaultPath(b, "cdn.example.com") {
		t.Fatal("path doesn't depend on domain and secret")
	}
	if len(p) != len("/api/")+10 || !strings.HasPrefix(p, "/api/") || strings.Trim(p[5:], "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
		t.Errorf("odd path %q", p)
	}
}
