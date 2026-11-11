package coreinstall

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const XrayCommit = "63ca2e0c1d31b89f67700c33842431f541fedd25"

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
	if err := verifyGoSource(XrayPath, XrayCommit); err != nil {
		return err
	}
	out, err := exec.Command(XrayPath, "version").Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "26.10.31") || !strings.Contains(string(out), "63ca2e0") {
		return fmt.Errorf("new protocols require pinned Xray 26.10.31 / 63ca2e0; existing engine was preserved")
	}
	return recordEngine(XrayPath, "https://github.com/XTLS/Xray-core", XrayCommit, "MPL-2.0", "version")
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
	if revision != commit || dirty {
		return fmt.Errorf("engine build does not match the clean pinned source commit; existing binary was preserved")
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
