package binary

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/releaseauth"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/version"
)

var httpClient = &http.Client{Timeout: 120 * time.Second}

// The rovunet binary comes from version.Repo's releases; the transport
// binaries from upstream SlipGate's, which builds them. A fork therefore only
// has to publish rovunet itself.
var (
	releaseBaseURL     = "https://github.com/" + version.Repo + "/releases"
	stableDownloadBase = releaseBaseURL + "/latest/download"
	repoAPI            = "https://api.github.com/repos/" + version.Repo + "/releases"
)

// OfflineDir, when set, makes EnsureInstalled copy binaries from this
// directory instead of downloading. Used for SCP/offline installs.
var OfflineDir string

// DownloadBase returns the base URL for rovunet binary downloads.
// Dev builds prefer the latest dev release; if none exists, fall back
// to the latest stable release. Production builds always use stable.
func DownloadBase() string {
	if version.ReleaseTag != "" {
		// Prefer latest dev release
		if tag := latestDevTag(); tag != "" {
			return releaseBaseURL + "/download/" + tag
		}
		// No dev release found — fall back to latest stable
		return stableDownloadBase
	}
	return stableDownloadBase
}

// latestDevTag queries GitHub for the most recent *-dev pre-release tag.
func latestDevTag() string {
	resp, err := httpClient.Get(repoAPI + "?per_page=10")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, releaseauth.MaxManifestSize+1))
	if err != nil {
		return ""
	}
	var releases []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
	}
	if json.Unmarshal(body, &releases) != nil {
		return ""
	}
	for _, r := range releases {
		if r.Prerelease && strings.HasSuffix(r.TagName, "-dev") {
			return r.TagName
		}
	}
	return ""
}

// errNotFound marks a release asset that doesn't exist.
var errNotFound = errors.New("HTTP 404")

// EnsureInstalled keeps an existing runtime. A missing runtime is staged from
// the reviewed, embedded release catalog, including offline installations.
func EnsureInstalled(name string) error {
	dest := filepath.Join(config.DefaultBinDir, name)
	if info, err := os.Lstat(dest); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("runtime destination is not a regular file")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := stageTransport(name, filepath.Dir(dest))
	if err != nil {
		return fmt.Errorf("stage pinned %s: %w", name, err)
	}
	defer os.Remove(tmp)
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	return safefile.SyncDir(filepath.Dir(dest))
}

// CheckUpdate checks GitHub releases for a newer version.
// Dev builds prefer the latest dev release; stable builds check latest stable.
func CheckUpdate() (newVersion string, downloadURL string, err error) {
	apiURL := repoAPI + "/latest"
	if version.ReleaseTag != "" {
		if tag := latestDevTag(); tag != "" {
			apiURL = repoAPI + "/tags/" + tag
		} else {
			// No dev release — check stable
			apiURL = repoAPI + "/latest"
		}
	}
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("release lookup HTTP %d", resp.StatusCode)
	}

	var release struct {
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		TagName    string `json:"tag_name"`
		Assets     []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, releaseauth.MaxManifestSize)).Decode(&release); err != nil {
		return "", "", err
	}
	if release.Draft || (!version.IsDev() && (release.Prerelease || strings.Contains(release.TagName, "-"))) {
		return "", "", errors.New("automatic stable update refused a draft or prerelease")
	}
	if err := rejectOlderRelease(version.Version, release.TagName); err != nil {
		return "", "", err
	}

	if release.TagName == version.Version || release.TagName == "v"+version.Version {
		return "", "", nil
	}

	// Find matching asset
	target := fmt.Sprintf("rovunet-%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, asset := range release.Assets {
		if asset.Name == target {
			return release.TagName, releaseBaseURL + "/download/" + release.TagName + "/" + target, nil
		}
	}

	return release.TagName, "", fmt.Errorf("no matching binary for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// Download stages authenticated release bytes in the destination filesystem.
// Neither the candidate nor an upstream checksum file is a trust anchor.
// Production keys are compiled from the independently reviewed keyring.
func Download(url, destDir string) (result string, err error) {
	a, err := authenticatedAsset(url)
	if err != nil {
		return "", err
	}
	for _, prefix := range []string{"rovunet-linux-", "slipgate-linux-", "xray-linux-", "amneziawg-go-linux-", "awg-linux-"} {
		if strings.HasPrefix(a.Name, prefix) && a.Kind != "executable" {
			return "", errors.New("manifest executable kind mismatch")
		}
	}
	tmp, err := os.CreateTemp(destDir, ".rovunet-update-*")
	if err != nil {
		return "", err
	}
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	if err = downloadLimited(url, tmp, a.Size); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err = verifyFile(tmp.Name(), a.Size, a.SHA256, runtime.GOARCH, a.Kind == "executable"); err != nil {
		return "", err
	}
	if a.Kind == "executable" {
		if err = os.Chmod(tmp.Name(), 0755); err != nil {
			return "", err
		}
	}
	return tmp.Name(), nil
}

func downloadToWriter(url string, w io.Writer) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("download %s: %w", url, errNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}
