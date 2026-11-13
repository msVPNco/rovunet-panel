//go:build windows

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/releaseauth"
)

func TestWindowsVaultRecoveryAndPurpose(t *testing.T) {
	vault, recovery := t.TempDir(), t.TempDir()
	original, err := initialize(vault, recovery)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = check(vault, recovery); err != nil {
		t.Fatal(err)
	}
	if _, err = initialize(vault, recovery); err == nil {
		t.Fatal("existing identity rotated")
	}
	recovered := t.TempDir()
	pub, err := recoverVault(recovered, recovery)
	if err != nil || pub.ID != original.ID {
		t.Fatal("independent recovery failed", err)
	}
	if _, err = check(recovered, recovery); err != nil {
		t.Fatal(err)
	}
	sealed, err := readBounded(filepath.Join(vault, "production.dpapi"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = unseal(sealed, []byte(purpose+".other")); err == nil {
		t.Fatal("wrong DPAPI purpose accepted")
	}
	sealed[len(sealed)/2] ^= 0x40
	if _, err = unseal(sealed, []byte(purpose)); err == nil {
		t.Fatal("tampered DPAPI accepted")
	}
	backup := filepath.Join(recovery, "production-backup.age")
	raw, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(backup, raw[:len(raw)/2], 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = recoverDER(recovery); err == nil {
		t.Fatal("truncated age recovery accepted")
	}
	if err = os.WriteFile(backup, raw, 0600); err != nil {
		t.Fatal(err)
	}
	different := t.TempDir()
	if _, err = initialize(t.TempDir(), different); err != nil {
		t.Fatal(err)
	}
	identity, err := os.ReadFile(filepath.Join(different, "recovery.agekey"))
	if err != nil {
		t.Fatal(err)
	}
	defer clear(identity)
	if err = os.WriteFile(filepath.Join(recovery, "recovery.agekey"), identity, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = recoverDER(recovery); err == nil {
		t.Fatal("wrong recovery key accepted")
	}
}

func TestWindowsVaultSignedManifestBoundary(t *testing.T) {
	vault, recovery, stage := t.TempDir(), t.TempDir(), t.TempDir()
	pub, err := initialize(vault, recovery)
	if err != nil {
		t.Fatal(err)
	}
	manifest := releaseauth.Manifest{Schema: 1, Environment: "production", KeyID: pub.ID,
		Repository: "msVPNco/rovunet-panel", Tag: "v1.21.2-review.2", SourceCommit: strings.Repeat("a", 40),
		Assets: []releaseauth.Asset{{Name: "install.sh", OS: "linux", Kind: "installer", Size: 10, SHA256: strings.Repeat("b", 64),
			Source: "https://github.com/msVPNco/rovunet-panel", SourceCommit: strings.Repeat("a", 40), License: "AGPL-3.0"}}}
	path := filepath.Join(stage, "manifest.json")
	sigPath := filepath.Join(stage, "signature")
	raw, _ := json.Marshal(manifest)
	os.WriteFile(path, raw, 0600)
	if _, err = signManifest(vault, path, sigPath, manifest.Tag); err != nil {
		t.Fatal(err)
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		t.Fatal(err)
	}
	key, der, _, err := loadPrivate(vault)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	defer clear(der)
	if !ed25519.Verify(key.Public().(ed25519.PublicKey), raw, sig) {
		t.Fatal("signature verification failed")
	}
	if _, err = signManifest(vault, path, sigPath, manifest.Tag); err == nil {
		t.Fatal("existing signature overwritten")
	}
	for _, mutate := range []func(*releaseauth.Manifest){
		func(m *releaseauth.Manifest) { m.Environment = "test" },
		func(m *releaseauth.Manifest) { m.Repository = "other/project" },
		func(m *releaseauth.Manifest) { m.Tag = "v1.21.3" },
		func(m *releaseauth.Manifest) { m.KeyID = strings.Repeat("c", 64) },
	} {
		m := manifest
		mutate(&m)
		body, _ := json.Marshal(m)
		os.WriteFile(path, body, 0600)
		if _, err = signManifest(vault, path, filepath.Join(t.TempDir(), "sig"), manifest.Tag); err == nil {
			t.Fatal("unapproved manifest signed")
		}
	}
	now, _ := os.ReadFile(sigPath)
	if !bytes.Equal(now, sig) {
		t.Fatal("prior signature changed")
	}
}
