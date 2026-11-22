package binary

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/releaseauth"
	"github.com/msVPNco/rovunet-panel/internal/version"
)

// The complete-setup service fixture preinstalls its engines. Exercise the
// missing-engine download instead, with a preview that has no release assets.
func TestPreviewDownloadsPinnedRuntimeAndRejectsInvalidRelease(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("release executables are Linux ELF files")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	id := sha256.Sum256(der)
	keyID := hex.EncodeToString(id[:])
	ring, err := json.Marshal(releaseauth.Keyring{Schema: 1, Keys: []releaseauth.Key{
		{ID: keyID, SPKI: base64.StdEncoding.EncodeToString(der), Purpose: "test"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := releaseauth.NewVerifier(ring, "test")
	if err != nil {
		t.Fatal(err)
	}
	oldVersion, oldTag, oldBase, oldVerifier := version.Version, version.ReleaseTag, releaseBaseURL, releaseVerifier
	t.Cleanup(func() {
		version.Version, version.ReleaseTag, releaseBaseURL, releaseVerifier = oldVersion, oldTag, oldBase, oldVerifier
	})
	version.Version, version.ReleaseTag = "2.0.3-preview.2", "dev-unpublished"
	releaseVerifier = func() (*releaseauth.Verifier, error) { return verifier, nil }
	asset := "xray-linux-" + runtime.GOARCH
	prefix := "/releases/download/" + RuntimeReleaseTag + "/"

	for _, scenario := range []string{"success", "missing-manifest", "missing-asset", "bad-signature", "wrong-tag", "wrong-repository", "wrong-architecture", "corrupt-asset", "truncated-asset", "production-rejects-test-key"} {
		t.Run(scenario, func(t *testing.T) {
			m := releaseauth.Manifest{Schema: 1, Environment: "test", KeyID: keyID,
				Repository: version.Repo, Tag: RuntimeReleaseTag, SourceCommit: strings.Repeat("a", 40),
				Assets: []releaseauth.Asset{{Name: asset, OS: runtime.GOOS, Arch: runtime.GOARCH,
					Kind: "executable", Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]),
					Source: "https://github.com/XTLS/Xray-core", SourceCommit: strings.Repeat("b", 40), License: "MPL-2.0"}},
			}
			switch scenario {
			case "wrong-tag":
				m.Tag = "v2.0.3-preview.2"
			case "wrong-repository":
				m.Repository = "untrusted/example"
			case "wrong-architecture":
				m.Assets[0].Arch = "arm64"
				if runtime.GOARCH == "arm64" {
					m.Assets[0].Arch = "amd64"
				}
			}
			raw, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			sig := ed25519.Sign(private, raw)
			if scenario == "bad-signature" {
				sig[0] ^= 1
			}
			var requests []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.URL.Path)
				switch r.URL.Path {
				case prefix + "RELEASE-MANIFEST.json":
					if scenario == "missing-manifest" {
						http.NotFound(w, r)
						return
					}
					w.Write(raw)
				case prefix + "RELEASE-MANIFEST.sig":
					w.Write(sig)
				case prefix + asset:
					switch scenario {
					case "missing-asset":
						http.NotFound(w, r)
					case "corrupt-asset":
						changed := bytes.Clone(data)
						changed[len(changed)-1] ^= 1
						w.Write(changed)
					case "truncated-asset":
						w.Write(data[:len(data)/2])
					default:
						w.Write(data)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			releaseBaseURL = srv.URL + "/releases"
			if scenario == "production-rejects-test-key" {
				releaseVerifier = releaseauth.Production
				defer func() { releaseVerifier = func() (*releaseauth.Verifier, error) { return verifier, nil } }()
			}
			dir := t.TempDir()
			file, downloadErr := DownloadRuntime(asset, dir)
			srv.Close()
			if scenario == "success" {
				if downloadErr != nil {
					t.Fatalf("preview could not acquire its reviewed runtime: %v", downloadErr)
				}
				got, err := os.ReadFile(file)
				if err != nil || !bytes.Equal(got, data) {
					t.Fatalf("download changed authenticated bytes: %v", err)
				}
			} else {
				if downloadErr == nil || file != "" {
					t.Fatalf("invalid runtime was accepted: file=%q err=%v", file, downloadErr)
				}
				if !strings.Contains(downloadErr.Error(), asset) || !strings.Contains(downloadErr.Error(), RuntimeReleaseTag) {
					t.Fatalf("error does not identify the runtime prerequisite: %v", downloadErr)
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("failed download left a staged candidate: %v %v", entries, err)
				}
			}
			if len(requests) == 0 {
				t.Fatal("runtime download was not exercised")
			}
			for _, request := range requests {
				if !strings.HasPrefix(request, prefix) {
					t.Fatalf("preview/latest fallback was requested: %s", request)
				}
			}
		})
	}
}
