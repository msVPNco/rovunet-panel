package version

import "fmt"

var (
	Version    = "1.6.3"
	Commit     = "unknown"
	ReleaseTag = "" // set via ldflags for dev builds (e.g. "dev-abc1234")

	// Repo is the GitHub repository whose releases carry the rovunet
	// binary: install.sh and `rovunet update` download from it, so a fork's
	// builds keep updating from the fork. Override with -ldflags -X.
	Repo = "msVPNco/rovunet-panel"
)

// UpstreamRepo builds the transport binaries (dnstt-server, caddy-naive, ...),
// which every Rovunet downloads from its releases.
const UpstreamRepo = "anonvector/slipgate"

func String() string {
	tag := ""
	if ReleaseTag != "" {
		tag = "-dev"
	}
	if Commit == "unknown" {
		return fmt.Sprintf("rovunet v%s%s", Version, tag)
	}
	return fmt.Sprintf("rovunet v%s%s (%s)", Version, tag, Commit)
}

// IsDev returns true if this is a dev channel build.
func IsDev() bool {
	return ReleaseTag != ""
}
