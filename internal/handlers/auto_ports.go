package handlers

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
)

// Random managed-core ports are chosen once at installation. Saved core
// options retain the selected port through restart, update and subscription
// refresh. DNS/53 and HTTPS/443 are base tunnel endpoints, not candidates.
const randomPortStart = 20000
const randomPortCount = 40000
const randomPortAttempts = 128

func securePortIndex(bound int) (int, error) {
	if bound <= 0 { return 0, fmt.Errorf("invalid random range") }
	n, err := rand.Int(rand.Reader, big.NewInt(int64(bound)))
	if err != nil { return 0, err }
	return int(n.Int64()), nil
}

// randomFreeCorePort uses a random high port for direct protocols. XHTTP may
// be placed behind CDNs whose proxy ports are restricted, so choose randomly
// only among the spec's CDN-safe ports instead of producing broken profiles.
// portFree must check the full TCP/UDP requirement of the protocol.
func randomFreeCorePort(spec completeCoreSpec, portFree func(int, string) error) (int, error) {
	return randomFreeCorePortWith(spec, portFree, securePortIndex)
}

func randomFreeCorePortWith(spec completeCoreSpec, portFree func(int, string) error,
	randIndex func(int) (int, error)) (int, error) {
	proto := coreinstall.Proto(spec.protocol)
	if spec.protocol != "xhttp" {
		for i := 0; i < randomPortAttempts; i++ {
			offset, err := randIndex(randomPortCount)
			if err != nil { return 0, fmt.Errorf("random core port selection: %w", err) }
			if offset < 0 || offset >= randomPortCount { return 0, fmt.Errorf("invalid random port index") }
			port := randomPortStart + offset
			if portFree(port, proto) == nil { return port, nil }
		}
	}
	// Safe fallback: the old documented recommended ports, shuffled instead of
	// tried deterministically. This is also the full candidate pool for XHTTP.
	ports := append([]int(nil), spec.ports...)
	for i := len(ports)-1; i > 0; i-- {
		j, err := randIndex(i + 1)
		if err != nil { return 0, fmt.Errorf("shuffling core ports: %w", err) }
		if j < 0 || j > i { return 0, fmt.Errorf("invalid shuffle index") }
		ports[i], ports[j] = ports[j], ports[i]
	}
	for _, port := range ports {
		if portFree(port, proto) == nil { return port, nil }
	}
	return 0, fmt.Errorf("no free random or recommended %s port for %s", proto, spec.protocol)
}
