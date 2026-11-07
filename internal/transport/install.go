package transport

import (
	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
)

// EnsureInstalled downloads the binary for a transport if not already present.
func EnsureInstalled(transport string) error {
	bin, ok := config.TransportBinaries[transport]
	if !ok {
		return nil
	}
	return binary.EnsureInstalled(bin)
}
