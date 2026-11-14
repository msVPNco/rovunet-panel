package coreinstall

import (
	"crypto/sha256"
	"debug/buildinfo"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const XrayCommit = "676eaf66f8f05d4d558ee7a74b4736623d3feeb9"

// Reviewed dependency fixes deliberately modify an upstream tree. Its VCS
// metadata alone cannot authenticate that patch set: exact embedded asset
// digests bind it to the reviewed source archives and patches instead.
//
//go:embed reviewed-go-runtime-pins.json
var reviewedGoRuntimePins []byte

type engineRecord struct {
	Source      string `json:"source"`
	Commit      string `json:"pinned_commit"`
	BuildCommit string `json:"verified_build_commit,omitempty"`
	SHA256      string `json:"sha256"`
	Version     string `json:"version"`
	License     string `json:"license"`
}

func recordEngine(path, source, commit, license string, args ...string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	out, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("engine version could not be read")
	}
	buildCommit := ""
	if info, err := buildinfo.ReadFile(path); err == nil {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != commit {
				return fmt.Errorf("engine source commit differs from the pinned runtime")
			}
			if s.Key == "vcs.revision" {
				buildCommit = s.Value
			}
		}
	}
	r := engineRecord{Source: source, Commit: commit, BuildCommit: buildCommit, SHA256: hex.EncodeToString(sum[:]), Version: strings.TrimSpace(string(out)), License: license}
	return writeJSON(path+".provenance.json", r)
}

func verifyXray() error {
	return verifyXrayFile(XrayPath)
}

func verifyXrayFile(path string) error {
	if err := verifyGoSource(path, XrayCommit); err != nil {
		return err
	}
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "26.11.9") || !strings.Contains(string(out), "676eaf6") {
		return fmt.Errorf("new protocols require reviewed Xray 26.11.9 / 676eaf6; existing engine was preserved")
	}
	return recordEngine(path, "https://github.com/XTLS/Xray-core", XrayCommit, "MPL-2.0", "version")
}

func recordAWG() error {
	if err := verifyAWGManifest(); err != nil {
		return err
	}
	if err := verifyGoSource(AWGGoPath, AWGGoCommit); err != nil {
		return err
	}
	if err := recordEngine(AWGGoPath, "https://github.com/amnezia-vpn/amneziawg-go", AWGGoCommit, "MIT", "--version"); err != nil {
		return err
	}
	return recordEngine(AWGToolPath, "https://github.com/amnezia-vpn/amneziawg-tools", AWGToolsCommit, "GPL-2.0", "--version")
}

func verifyGoSource(path, commit string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return fmt.Errorf("engine has no verifiable Go build metadata")
	}
	var revision string
	dirty := false
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
		}
		if s.Key == "vcs.modified" && s.Value == "true" {
			dirty = true
		}
	}
	if revision != commit {
		return fmt.Errorf("engine build does not match the clean pinned source commit; existing binary was preserved")
	}
	var pins []struct {
		Commit string `json:"commit"`
		Arch   string `json:"arch"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(reviewedGoRuntimePins, &pins); err != nil {
		return fmt.Errorf("invalid embedded runtime review pins")
	}
	matchedCommit := false
	for _, pin := range pins {
		if pin.Commit != commit {
			continue
		}
		matchedCommit = true
		if pin.Arch != runtime.GOARCH {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) == pin.SHA256 {
			return nil
		}
	}
	if matchedCommit || dirty {
		return fmt.Errorf("engine checksum is not an exact reviewed build; existing binary was preserved")
	}
	return nil
}

func awgManifestPath() string {
	return filepath.Join(filepath.Dir(AWGGoPath), "rovunet-awg-provenance.json")
}

func verifyAWGManifest() error {
	b, err := os.ReadFile(awgManifestPath())
	if err != nil {
		return fmt.Errorf("verified AWG runtime manifest is required: %w", err)
	}
	var manifest struct {
		Engines []struct{ Asset, Commit, SHA256 string }
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		return err
	}
	for _, item := range []struct{ name, path, commit string }{{"amneziawg-go", AWGGoPath, AWGGoCommit}, {"awg", AWGToolPath, AWGToolsCommit}} {
		asset := item.name + "-linux-" + runtime.GOARCH
		data, err := os.ReadFile(item.path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		found := false
		for _, e := range manifest.Engines {
			if e.Asset == asset && e.Commit == item.commit && e.SHA256 == hex.EncodeToString(sum[:]) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("AWG runtime checksum/source manifest mismatch for %s", item.name)
		}
	}
	return nil
}
