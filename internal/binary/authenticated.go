package binary

import (
	"crypto/sha256"
	"debug/elf"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/releaseauth"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/version"
)

// No environment variable can replace the compiled production trust root.
var releaseVerifier = releaseauth.Production

//go:embed transport-catalog.json
var transportCatalog []byte

type pinnedTransport struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func transportAsset(name, goos, arch string) (pinnedTransport, string, error) {
	switch name {
	case "dnstt-server", "slipstream-server", "vaydns-server", "caddy-naive":
	default:
		return pinnedTransport{}, "", errors.New("unknown transport binary")
	}
	var catalog struct {
		Schema        int               `json:"schema"`
		Repository    string            `json:"repository"`
		Tag           string            `json:"tag"`
		ReleaseCommit string            `json:"release_commit"`
		Assets        []pinnedTransport `json:"assets"`
	}
	if err := json.Unmarshal(transportCatalog, &catalog); err != nil {
		return pinnedTransport{}, "", err
	}
	for _, a := range catalog.Assets {
		if a.Name == name+"-"+goos+"-"+arch {
			return a, "https://github.com/" + catalog.Repository + "/releases/download/" + catalog.Tag + "/" + a.Name, nil
		}
	}
	return pinnedTransport{}, "", fmt.Errorf("no pinned %s runtime for %s/%s", name, goos, arch)
}

// RuntimeDownloadBase binds runtime bytes to the installed panel release;
// changing an upstream or panel "latest" must not change this version's cores.
func RuntimeDownloadBase() string { return releaseBaseURL + "/download/v" + version.Version }

func fetchSmall(url string, limit int64) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, errors.New("release metadata request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release metadata HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, errors.New("release metadata response was incomplete")
	}
	if int64(len(b)) > limit {
		return nil, errors.New("release metadata exceeds size limit")
	}
	return b, nil
}

func authenticatedAsset(downloadURL string) (releaseauth.Asset, error) {
	v, err := releaseVerifier()
	if err != nil {
		return releaseauth.Asset{}, err
	}
	u, err := url.Parse(downloadURL)
	if err != nil || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return releaseauth.Asset{}, errors.New("invalid release asset URL")
	}
	base, name := strings.TrimSuffix(downloadURL, "/"+path.Base(u.Path)), path.Base(u.Path)
	tag := ""
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if p == "download" && i+2 == len(parts)-1 {
			tag = parts[i+1]
		}
	}
	raw, err := fetchSmall(base+"/RELEASE-MANIFEST.json", releaseauth.MaxManifestSize)
	if err != nil {
		return releaseauth.Asset{}, err
	}
	sig, err := fetchSmall(base+"/RELEASE-MANIFEST.sig", 64)
	if err != nil {
		return releaseauth.Asset{}, err
	}
	m, err := v.Verify(raw, sig, version.Repo, tag)
	if err != nil {
		return releaseauth.Asset{}, err
	}
	return m.Find(name, runtime.GOOS, runtime.GOARCH)
}

// verifyFile does not execute a candidate, including to ask for its version.
func verifyFile(file string, size int64, digest, arch string, executable bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, size+1))
	if err != nil {
		return err
	}
	if n != size || hex.EncodeToString(h.Sum(nil)) != digest {
		return errors.New("release asset size or authenticated checksum mismatch")
	}
	if executable {
		e, err := elf.Open(file)
		if err != nil {
			return errors.New("release executable is not a valid ELF")
		}
		defer e.Close()
		want := map[string]elf.Machine{"amd64": elf.EM_X86_64, "arm64": elf.EM_AARCH64}[arch]
		if e.Class != elf.ELFCLASS64 || e.Data != elf.ELFDATA2LSB || e.Machine != want || (e.Type != elf.ET_EXEC && e.Type != elf.ET_DYN) {
			return errors.New("release executable architecture is incorrect")
		}
		load := false
		for _, p := range e.Progs {
			if p.Type == elf.PT_LOAD && p.Filesz > 0 && p.Off <= uint64(size) && p.Filesz <= uint64(size)-p.Off {
				load = true
			}
		}
		if !load {
			return errors.New("release executable has no valid load segment")
		}
	}
	return nil
}

func stageTransport(name, destDir string) (string, error) {
	a, downloadURL, err := transportAsset(name, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(destDir, ".rovunet-transport-*")
	if err != nil {
		return "", err
	}
	good := false
	defer func() {
		f.Close()
		if !good {
			os.Remove(f.Name())
		}
	}()
	if OfflineDir != "" {
		// The convenience aliases do not weaken the pinned digest/architecture.
		var src *os.File
		for _, n := range []string{a.Name, name + "-" + runtime.GOARCH, name} {
			src, err = os.Open(filepath.Join(OfflineDir, n))
			if err == nil {
				break
			}
		}
		if err != nil {
			return "", errors.New("pinned runtime is missing from offline directory")
		}
		_, err = io.Copy(f, io.LimitReader(src, a.Size+1))
		src.Close()
	} else {
		err = downloadLimited(downloadURL, f, a.Size)
	}
	if err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := verifyFile(f.Name(), a.Size, a.SHA256, runtime.GOARCH, true); err != nil {
		return "", err
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		return "", err
	}
	good = true
	return f.Name(), nil
}

// UpdateInstalled stages and validates a complete pinned transport while its
// healthy binary remains present. The previous complete binary is retained.
func UpdateInstalled(name string) error {
	dest := filepath.Join(config.DefaultBinDir, name)
	a, _, err := transportAsset(name, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	// An idempotent retry must keep the last pre-upgrade recovery copy.
	if verifyFile(dest, a.Size, a.SHA256, runtime.GOARCH, true) == nil {
		return nil
	}
	tmp, err := stageTransport(name, filepath.Dir(dest))
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := safefile.Copy(dest, dest+".bak", 0755); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	return safefile.SyncDir(filepath.Dir(dest))
}

func downloadLimited(downloadURL string, w io.Writer, size int64) error {
	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		return errors.New("release asset request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("release asset HTTP %d", resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, size+1))
	if err != nil || n != size {
		return errors.New("release asset response has an incomplete or unexpected size")
	}
	return nil
}
