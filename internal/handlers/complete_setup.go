package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/backup"
	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"golang.org/x/term"
)

type completeCoreSpec struct {
	name, protocol string
	ports          []int
}

// Names remain stable across fresh installation and upgrades from older tags.
var completeCoreSpecs = []completeCoreSpec{
	{"reality", "reality", []int{9443, 2096, 8880}},
	{"reality2", "reality", []int{2087, 2053, 8443}},
	{"xhttp", "xhttp", []int{2096, 2087, 8880}},
	{"hysteria2", "hysteria2", []int{8443, 4443, 2096}},
	{"wireguard", "wireguard", []int{51820, 51821, 51822}},
	{"managed-awg", "amneziawg", []int{51840, 51841, 51842}},
	{"managed-ss2022", "ss2022", []int{28444, 28445, 28446}},
	{"managed-trojan", "trojan", []int{30444, 30445, 30446}},
	{"managed-vlessenc", "vlessenc", []int{31444, 31445, 31446}},
}

// No plain public SOCKS5 listener is needed for the 24-profile setup.
func completeTransports() []string {
	return []string{config.TransportDNSTT, config.TransportSlipstream, config.TransportVayDNS,
		config.TransportSoar, config.TransportNaive, config.TransportStunTLS, config.TransportVLESS, config.TransportSSH}
}

func missingCompleteBackends(cfg *config.Config, transport string) []string {
	want := []string{config.BackendSOCKS}
	switch transport {
	case config.TransportDNSTT, config.TransportSlipstream, config.TransportVayDNS:
		want = []string{config.BackendSOCKS, config.BackendSSH}
	case config.TransportSoar:
		want = []string{config.BackendDirect, config.BackendSSH}
	default:
		for _, tunnel := range cfg.Tunnels {
			if tunnel.Transport == transport {
				return nil
			}
		}
		return want
	}
	have := map[string]bool{}
	for _, tunnel := range cfg.Tunnels {
		if tunnel.Transport == transport {
			have[tunnel.Backend] = true
		}
	}
	var missing []string
	for _, backend := range want {
		if !have[backend] {
			missing = append(missing, backend)
		}
	}
	return missing
}

func completeBaseTunnels(cfg *config.Config) bool {
	for _, transport := range completeTransports() {
		if len(missingCompleteBackends(cfg, transport)) != 0 {
			return false
		}
	}
	return true
}

func completeAfterUpdate(ctx *actions.Context, cfg *config.Config) error {
	if !completeBaseTunnels(cfg) {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("missing base tunnels need DNS/domain settings; run 'rovunet install' interactively to complete the setup without removing existing users or tunnels")
		}
		ctx.Output.Info("Completing missing base tunnels; existing users and tunnels are kept. Supply the requested DNS/domain settings.")
		return handleSystemInstall(ctx)
	}
	return EnsureCompleteSetup(cfg, ctx.Output)
}

// HandleCompleteSetup is also the retry-only CLI entry point.
func HandleCompleteSetup(ctx *actions.Context) error {
	if err := completeAfterUpdate(ctx, ctx.Config.(*config.Config)); err != nil {
		return err
	}
	return CompleteBootstrap()
}

type completeSetupOps struct {
	list     func() ([]coreinstall.Meta, error)
	install  func(string, coreplan.Options, []config.UserConfig) (string, error)
	portFree func(int, string) error
	cert     func(string) (string, string, bool)
	backup   func(*config.Config) (string, error)
	reality  func(*coreplan.Options, string) error
}

func setupBackup(cfg *config.Config) (string, error) {
	return setupBackupAt(cfg, filepath.Join(config.DefaultConfigDir, "setup-backups"))
}

func setupBackupAt(cfg *config.Config, dir string) (string, error) {
	text, skipped, err := encodeBackup(cfg)
	if err != nil {
		return "", err
	}
	if len(skipped) != 0 {
		return "", fmt.Errorf("automatic setup backup would omit %d files", len(skipped))
	}
	if _, err := backup.Decode(text); err != nil {
		return "", fmt.Errorf("automatic setup backup validation failed: %w", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "before-complete-*.backup")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, writeErr := f.WriteString(text)
	syncErr := f.Sync()
	closeErr := f.Close()
	return path, errors.Join(writeErr, syncErr, closeErr)
}

// EnsureCompleteSetup adds missing cores only. Existing names, stopped services,
// credentials and runtime files are not adopted or replaced by this operation.
func EnsureCompleteSetup(cfg *config.Config, out actions.OutputWriter) error {
	if err := os.MkdirAll(config.DefaultConfigDir, 0750); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(config.DefaultConfigDir, ".complete-setup.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	// Refresh after taking the lock; another setup may have finished meanwhile.
	if err := cfg.Reload(); err != nil {
		return err
	}
	ops := completeSetupOps{
		list: coreinstall.List, install: coreinstall.InstallForUsers, portFree: coreinstall.PortFree,
		cert: func(domain string) (string, string, bool) {
			crt, key, found := waitCaddyCert(out, domain, 90*time.Second)
			if !found || validSetupCertificate(crt, key, domain) != nil {
				return "", "", false
			}
			return crt, key, true
		},
		backup: setupBackup,
		reality: func(o *coreplan.Options, avoid string) error {
			if avoid != "" {
				_, err := coreplan.AutoRealityOther(o, avoid)
				return err
			}
			_, err := coreplan.AutoReality(o)
			return err
		},
	}
	return ensureCompleteSetupWith(cfg, out, ops)
}

func validSetupCertificate(crt, key, domain string) error {
	pair, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		return fmt.Errorf("certificate/key pair is unreadable or mismatched")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return err
	}
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return err
		}
		intermediates.AddCert(cert)
	}
	_, err = leaf.Verify(x509.VerifyOptions{DNSName: domain, Intermediates: intermediates})
	return err
}

func ensureCompleteSetupWith(cfg *config.Config, out actions.OutputWriter, ops completeSetupOps) error {
	list, err := ops.list()
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	names := map[string]bool{}
	realities := 0
	avoid := ""
	for _, m := range list {
		names[m.Name] = true
		existing[m.Protocol] = true
		if m.Protocol == "reality" {
			realities++
			var o coreplan.Options
			if data, err := os.ReadFile(filepath.Join(coreinstall.Root, m.Name, "options.json")); err == nil && json.Unmarshal(data, &o) == nil && o.SNI != "" {
				avoid = o.SNI
			}
		}
	}
	eligible := false
	for _, u := range cfg.Users {
		if !u.MeteredOnly() && !u.Expired(time.Now()) {
			eligible = true
		}
	}
	host, hostOK := clientcfg.ServerAddress(cfg)
	domain := naiveDomain(cfg)
	crt, key := "", ""
	certChecked, certOK := false, false
	backedUp := false
	var problems []error
	for _, spec := range completeCoreSpecs {
		name := spec.name
		if spec.protocol == "reality" {
			if (spec.name == "reality" && realities >= 1) || (spec.name == "reality2" && realities >= 2) {
				continue
			}
		} else if existing[spec.protocol] {
			continue
		}
		if names[name] && spec.protocol == "reality" && !names["reality"] {
			name = "reality"
		}
		if !eligible && (spec.protocol == "amneziawg" || spec.protocol == "ss2022" || spec.protocol == "trojan" || spec.protocol == "vlessenc" || spec.protocol == "wireguard" || spec.protocol == "hysteria2") {
			out.Info("Automatic setup: " + spec.protocol + " excluded for traffic-limited/expired accounts")
			continue
		}
		if !hostOK || host == "" {
			problems = append(problems, fmt.Errorf("%s: public server address is unavailable", spec.protocol))
			continue
		}
		o := coreplan.Options{Protocol: spec.protocol, Host: host}
		for _, port := range spec.ports {
			if ops.portFree(port, coreinstall.Proto(spec.protocol)) == nil {
				o.Port = port
				break
			}
		}
		if o.Port == 0 {
			problems = append(problems, fmt.Errorf("%s: candidate ports are occupied", spec.protocol))
			continue
		}
		if spec.protocol == "xhttp" || spec.protocol == "hysteria2" || spec.protocol == "trojan" {
			if !certChecked {
				certChecked = true
				if domain != "" {
					crt, key, certOK = ops.cert(domain)
				}
			}
			if !certOK {
				problems = append(problems, fmt.Errorf("%s: NaiveProxy domain with a valid certificate is required", spec.protocol))
				continue
			}
			o.Host, o.SNI, o.Cert, o.Key = domain, domain, crt, key
		}
		if spec.protocol == "reality" {
			if err := ops.reality(&o, avoid); err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", spec.name, err))
				continue
			}
		}
		if spec.protocol == "amneziawg" {
			o.Subnet = "10.203.72.0/24"
		}
		o.DedicatedXray = spec.protocol == "ss2022" || spec.protocol == "trojan" || spec.protocol == "vlessenc"
		o.HysteriaSalamander, o.HysteriaObfsConfigured = spec.protocol == "hysteria2", spec.protocol == "hysteria2"
		if !backedUp {
			path, err := ops.backup(cfg)
			if err != nil {
				return fmt.Errorf("automatic setup stopped before creating cores: %w", err)
			}
			out.Info("Validated pre-setup backup: " + path)
			backedUp = true
		}
		for _, warning := range coreplan.Warnings(spec.protocol) {
			out.Warning(warning)
		}
		out.Info(fmt.Sprintf("Automatic setup: installing %s on %d/%s", spec.protocol, o.Port, coreinstall.Proto(spec.protocol)))
		if _, err := ops.install(name, o, cfg.Users); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", spec.protocol, err))
			continue
		}
		existing[spec.protocol] = true
		names[name] = true
		if spec.protocol == "reality" {
			realities++
			avoid = o.SNI
		}
	}
	if err := WriteSubscriptions(cfg, out); err != nil {
		problems = append(problems, err)
	}
	if err := reportCompleteProfiles(cfg, out); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

func reportCompleteProfiles(cfg *config.Config, out actions.OutputWriter) error {
	var problems []error
	for _, u := range cfg.Users {
		if u.Expired(time.Now()) {
			continue
		}
		android, err := clientexport.Build(cfg, u, cfg.Tunnels, clientexport.Options{Client: "android"})
		if err != nil {
			problems = append(problems, err)
			continue
		}
		windows, err := clientexport.Build(cfg, u, cfg.Tunnels, clientexport.Options{Client: "windows"})
		if err != nil {
			problems = append(problems, err)
			continue
		}
		out.Info(fmt.Sprintf("Subscription %s: Android=%d Windows=%d; existing URL retained", u.Username, len(android), len(windows)))
		if u.MeteredOnly() {
			out.Info("Traffic-limited account: restricted protocol set retained")
			continue
		}
		required := 24
		if u.ExpiresAt != "" {
			required--
		} // shared-key Soar Direct is excluded for timed accounts
		if len(android) < required || len(windows) < required {
			problems = append(problems, fmt.Errorf("account %s: complete setup pending (Android %d/%d, Windows %d/%d); check missing/disabled tunnels, DNS, certificates and core errors above", u.Username, len(android), required, len(windows), required))
		}
	}
	return errors.Join(problems...)
}
