package certs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

// CaddyDataDirs are the places Caddy keeps the certificates it obtained.
// Caddy runs as root, so its data is under root's home; with WARP enabled it
// uses a data directory inside the tunnel's own directory. Variable for tests.
var CaddyDataDirs = []string{
	"/root/.local/share/caddy",
	"/var/lib/caddy/.local/share/caddy",
	"/var/lib/caddy",
}

// FindCaddy returns the certificate and private key Caddy holds for domain,
// as paths. ok is false until Caddy has obtained one.
func FindCaddy(domain string) (crt, key string, ok bool) {
	if domain == "" || strings.ContainsAny(domain, `/\`) || strings.Contains(domain, "..") {
		return "", "", false
	}
	bases := append([]string(nil), CaddyDataDirs...)
	if m, err := filepath.Glob(filepath.Join(config.DefaultTunnelDir, "*", ".caddy", "caddy")); err == nil {
		bases = append(bases, m...)
	}
	for _, base := range bases {
		matches, _ := filepath.Glob(filepath.Join(base, "certificates", "*", domain, domain+".crt"))
		for _, c := range matches {
			k := strings.TrimSuffix(c, ".crt") + ".key"
			if _, err := os.Stat(k); err == nil {
				return c, k, true
			}
		}
	}
	return "", "", false
}
