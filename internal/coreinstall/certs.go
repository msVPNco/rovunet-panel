package coreinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

// XHTTP and Hysteria 2 read their TLS certificate only when they start, so a
// renewed certificate (Let's Encrypt renews about every 60 days) is not served
// until the core restarts. RestartIfCertChanged, run daily by a systemd timer,
// does that restart.

const certSumFile = "cert.sha256"

var certFileRe = regexp.MustCompile(`"certificateFile"\s*:\s*"([^"]+)"`)

// restartCore restarts a core's service. Variable for tests.
var restartCore = service.Restart

// certPath is the certificate file a core serves, or "" when it has none.
func certPath(name string) string {
	data, err := os.ReadFile(filepath.Join(Root, name, "server.json"))
	if err != nil {
		return ""
	}
	if m := certFileRe.FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return ""
}

func certSum(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), true
}

// RecordCert remembers the certificate a core has just started with.
func RecordCert(name string) {
	p := certPath(name)
	if p == "" {
		return
	}
	if sum, ok := certSum(p); ok {
		_ = os.WriteFile(filepath.Join(Root, name, certSumFile), []byte(sum), 0600)
	}
}

// RestartIfCertChanged restarts every core whose certificate file changed
// since the core started, and returns their names. A core with no recorded
// state is recorded as it is, not restarted.
func RestartIfCertChanged() ([]string, error) {
	cores, err := List()
	if err != nil {
		return nil, err
	}
	var restarted []string
	var firstErr error
	for _, c := range cores {
		p := certPath(c.Name)
		if p == "" {
			continue
		}
		sum, ok := certSum(p)
		if !ok {
			continue
		}
		sumPath := filepath.Join(Root, c.Name, certSumFile)
		old, err := os.ReadFile(sumPath)
		if err != nil {
			_ = os.WriteFile(sumPath, []byte(sum), 0600)
			continue
		}
		if string(old) == sum {
			continue
		}
		if err := restartCore(ServiceName(c.Name)); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		_ = os.WriteFile(sumPath, []byte(sum), 0600)
		restarted = append(restarted, c.Name)
	}
	return restarted, firstErr
}

// HasCertCores reports whether any installed core serves a TLS certificate.
func HasCertCores() bool {
	cores, _ := List()
	for _, c := range cores {
		if certPath(c.Name) != "" {
			return true
		}
	}
	return false
}

const certTimerService = `[Unit]
Description=Restart Rovunet Xray cores after their TLS certificate was renewed

[Service]
Type=oneshot
ExecStart=%s core-certs
`

const certTimer = `[Unit]
Description=Check Rovunet Xray cores' TLS certificates twice a day

[Timer]
OnCalendar=*-*-* 04,16:00:00
RandomizedDelaySec=30min
Persistent=true

[Install]
WantedBy=timers.target
`

// EnsureCertTimer installs the timer that runs RestartIfCertChanged, when
// some core serves a certificate. It is idempotent.
func EnsureCertTimer() error {
	if !HasCertCores() {
		return nil
	}
	self, err := config.SelfPath()
	if err != nil {
		return err
	}
	dir := "/etc/systemd/system"
	svc := []byte(fmt.Sprintf(certTimerService, self))
	if err := os.WriteFile(filepath.Join(dir, "rovunet-core-certs.service"), svc, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "rovunet-core-certs.timer"), []byte(certTimer), 0644); err != nil {
		return err
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("systemctl", "enable", "--now", "rovunet-core-certs.timer").CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
