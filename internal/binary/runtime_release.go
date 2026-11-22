package binary

import (
	"fmt"
	"path"
)

// RuntimeReleaseTag identifies the published assets matching the reviewed
// runtime pins in coreinstall. It is deliberately independent of the panel's
// version: a locally built preview does not have its own published runtimes.
// Change this only with the reviewed runtime hashes and runtime acceptance.
// There is no environment override or fallback to a moving "latest" release.
const RuntimeReleaseTag = "v2.0.2"

func RuntimeDownloadBase() string {
	return releaseBaseURL + "/download/" + RuntimeReleaseTag
}

// DownloadRuntime retains the normal production manifest, signature, size,
// checksum and architecture verification while identifying failed prerequisites
// in install/complete-setup output. It never installs or executes the candidate.
func DownloadRuntime(asset, destDir string) (string, error) {
	if asset == "" || asset == "." || asset == ".." || path.Base(asset) != asset {
		return "", fmt.Errorf("invalid managed runtime asset name")
	}
	file, err := Download(RuntimeDownloadBase()+"/"+asset, destDir)
	if err != nil {
		return "", fmt.Errorf("managed runtime %s from release %s: %w", asset, RuntimeReleaseTag, err)
	}
	return file, nil
}
