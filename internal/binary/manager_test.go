package binary

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/releaseauth"
	"github.com/msVPNco/rovunet-panel/internal/version"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestDownloadNotFound: only a missing asset reports errNotFound, the one
// error EnsureInstalled falls back to the stable release on.
func TestDownloadNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.NotFound(w, r)
		case "/broken":
			http.Error(w, "oops", http.StatusInternalServerError)
		default:
			io.WriteString(w, "binary")
		}
	}))
	defer srv.Close()

	if err := downloadToWriter(srv.URL+"/missing", io.Discard); !errors.Is(err, errNotFound) {
		t.Errorf("missing asset: got %v, want errNotFound", err)
	}
	if err := downloadToWriter(srv.URL+"/broken", io.Discard); err == nil || errors.Is(err, errNotFound) {
		t.Errorf("server error: got %v, want a non-404 error", err)
	}
	if err := downloadToWriter(srv.URL+"/ok", io.Discard); err != nil {
		t.Errorf("existing asset: %v", err)
	}
}

// sumsServer returns an httptest server serving assetName at its own URL
// path and a SHA256SUMS listing a digest for it — correct unless
// wrongDigest is set, in which case the listed digest is deliberately
// wrong so callers can test rejection.
func sumsServer(t *testing.T, assetName, content string, wrongDigest bool) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(sum[:])
	if wrongDigest {
		digest = strings.Repeat("0", 64)
	}
	sums := fmt.Sprintf("%s  %s\n", digest, assetName)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKIXPublicKey(pub)
	id := sha256.Sum256(der)
	keyID := hex.EncodeToString(id[:])
	ring, _ := json.Marshal(releaseauth.Keyring{Schema: 1, Keys: []releaseauth.Key{{ID: keyID, SPKI: base64.StdEncoding.EncodeToString(der), Purpose: "test"}}})
	v, err := releaseauth.NewVerifier(ring, "test")
	if err != nil {
		t.Fatal(err)
	}
	old := releaseVerifier
	releaseVerifier = func() (*releaseauth.Verifier, error) { return v, nil }
	t.Cleanup(func() { releaseVerifier = old })
	m := releaseauth.Manifest{Schema: 1, Environment: "test", KeyID: keyID, Repository: version.Repo, Tag: "v1.21.2", SourceCommit: strings.Repeat("a", 40), Assets: []releaseauth.Asset{{Name: assetName, OS: "linux", Arch: "amd64", Kind: "executable", Size: int64(len(content)), SHA256: digest, Source: "https://github.com/" + version.Repo, SourceCommit: strings.Repeat("a", 40), License: "MIT"}}}
	body, _ := json.Marshal(m)
	signature := ed25519.Sign(priv, body)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + assetName:
			io.WriteString(w, content)
		case "/SHA256SUMS":
			io.WriteString(w, sums)
		case "/RELEASE-MANIFEST.json":
			w.Write(body)
		case "/RELEASE-MANIFEST.sig":
			w.Write(signature)
		default:
			http.NotFound(w, r)
		}
	}))
}

// TestDownloadVerifiesChecksum is the regression test for binary downloads
// (this project's own rovunet binary, used for self-update) installing
// without any checksum verification despite SHA256SUMS being published
// alongside every release.
func TestDownloadVerifiesChecksum(t *testing.T) {
	const asset = "rovunet-linux-amd64"
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	srv := sumsServer(t, asset, content, false)
	defer srv.Close()

	destDir := t.TempDir()
	path, err := Download(srv.URL+"/"+asset, destDir)
	if err != nil {
		t.Fatalf("Download with a correct checksum: %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != content {
		t.Errorf("downloaded content = %q, want %q", got, content)
	}
}

// TestDownloadRejectsChecksumMismatch confirms a SHA256SUMS entry that
// doesn't match the downloaded bytes is treated as a fatal error, not a
// warning — the whole point is that Download never hands back an
// unverified binary.
func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	const asset = "rovunet-linux-amd64"
	srv := sumsServer(t, asset, "fake binary content", true)
	defer srv.Close()

	destDir := t.TempDir()
	path, err := Download(srv.URL+"/"+asset, destDir)
	if err == nil {
		os.Remove(path)
		t.Fatal("expected Download to reject a checksum mismatch, got no error")
	}

	entries, _ := os.ReadDir(destDir)
	if len(entries) != 0 {
		t.Errorf("expected no leftover temp file after a rejected download, found %v", entries)
	}
}

// TestDownloadRejectsMissingSHA256Sums confirms Download fails closed when
// SHA256SUMS itself can't be fetched, rather than skipping verification.
func TestDownloadRejectsMissingManifest(t *testing.T) {
	trust := sumsServer(t, "rovunet-linux-amd64", "fixture", false)
	defer trust.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rovunet-linux-amd64" {
			io.WriteString(w, "fake binary content")
			return
		}
		http.NotFound(w, r) // no SHA256SUMS on this server
	}))
	defer srv.Close()

	destDir := t.TempDir()
	if _, err := Download(srv.URL+"/rovunet-linux-amd64", destDir); err == nil {
		t.Fatal("expected Download to fail when SHA256SUMS is unavailable, got no error")
	}
}
