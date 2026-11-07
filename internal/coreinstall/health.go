package coreinstall

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CertTimerName is the systemd timer that restarts cores after a renewal.
const CertTimerName = "rovunet-core-certs.timer"

// CertExpiry returns when the TLS certificate a core serves expires. It
// returns ok=false when the core serves no certificate.
func CertExpiry(name string) (t time.Time, ok bool, err error) {
	p := certPath(name)
	if p == "" {
		return time.Time{}, false, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return time.Time{}, true, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return time.Time{}, true, fmt.Errorf("%s: no PEM certificate", p)
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, true, err
	}
	return crt.NotAfter, true, nil
}

// CertTimerEnabled reports whether the certificate-check timer is enabled.
func CertTimerEnabled() bool {
	out, _ := exec.Command("systemctl", "is-enabled", CertTimerName).Output()
	return strings.TrimSpace(string(out)) == "enabled"
}
