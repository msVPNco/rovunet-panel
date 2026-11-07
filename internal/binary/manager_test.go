package binary

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + assetName:
			io.WriteString(w, content)
		case "/SHA256SUMS":
			io.WriteString(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
}

// TestDownloadVerifiesChecksum is the regression test for binary downloads
// (this project's own slipgate binary, used for self-update) installing
// without any checksum verification despite SHA256SUMS being published
// alongside every release.
func TestDownloadVerifiesChecksum(t *testing.T) {
	const asset = "slipgate-linux-amd64"
	const content = "fake binary content"
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
	const asset = "slipgate-linux-amd64"
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
func TestDownloadRejectsMissingSHA256Sums(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slipgate-linux-amd64" {
			io.WriteString(w, "fake binary content")
			return
		}
		http.NotFound(w, r) // no SHA256SUMS on this server
	}))
	defer srv.Close()

	destDir := t.TempDir()
	if _, err := Download(srv.URL+"/slipgate-linux-amd64", destDir); err == nil {
		t.Fatal("expected Download to fail when SHA256SUMS is unavailable, got no error")
	}
}
