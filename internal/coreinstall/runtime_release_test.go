package coreinstall

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/releaseauth"
	"github.com/msVPNco/rovunet-panel/internal/version"
)

// Keep the selected download release and every exact runtime pin together.
// The fixture is the unmodified public production manifest/signature, not a
// test-signed imitation. Panel version bumps must not invalidate this contract.
func TestPinnedRuntimeReleaseMatchesProductionManifest(t *testing.T) {
	dir := filepath.Join("testdata", "runtime-release")
	raw, err := os.ReadFile(filepath.Join(dir, "RELEASE-MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(filepath.Join(dir, "RELEASE-MANIFEST.sig.base64"))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	v, err := releaseauth.Production()
	if err != nil {
		t.Fatal(err)
	}
	m, err := v.Verify(raw, sig, version.Repo, binary.RuntimeReleaseTag)
	if err != nil {
		t.Fatalf("runtime release is not authenticated: %v", err)
	}
	assets := map[string]releaseauth.Asset{}
	for _, asset := range m.Assets {
		assets[asset.Name] = asset
	}
	var pins []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(reviewedReleaseRuntimePins, &pins); err != nil || len(pins) == 0 {
		t.Fatalf("missing release pins: %v", err)
	}
	for _, pin := range pins {
		asset, ok := assets[pin.Name]
		if !ok || asset.Size != pin.Size || asset.SHA256 != pin.SHA256 {
			t.Errorf("%s pins do not match the selected authenticated release", pin.Name)
		}
	}
	var goPins []struct {
		Commit string `json:"commit"`
		Arch   string `json:"arch"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(reviewedGoRuntimePins, &goPins); err != nil || len(goPins) == 0 {
		t.Fatalf("missing Go runtime pins: %v", err)
	}
	for _, pin := range goPins {
		matched := false
		for _, asset := range m.Assets {
			if asset.SourceCommit == pin.Commit && asset.Arch == pin.Arch && asset.SHA256 == pin.SHA256 && asset.Kind == "executable" {
				matched = true
			}
		}
		if !matched {
			t.Errorf("Go runtime pin %s/%s has no matching published executable", pin.Commit, pin.Arch)
		}
	}
	for name, arches := range supplementalHashes {
		for arch, digest := range arches {
			asset, ok := assets[name+"-linux-"+arch]
			if !ok || asset.SHA256 != digest || asset.Kind != "executable" {
				t.Errorf("%s/%s supplemental pin differs from the selected release", name, arch)
			}
		}
	}
}
