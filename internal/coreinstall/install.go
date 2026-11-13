// Package coreinstall turns a coreplan bundle into a managed Xray service:
// validated config on disk, a systemd unit, and an opened firewall port.
// Installs live under their own directory and never touch config.json, so a
// failed or removed core cannot affect panel-managed tunnels.
package coreinstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/network"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

// Root holds one directory per installed core. Variable for tests.
var Root = filepath.Join(config.DefaultConfigDir, "core")

// XrayPath is where the verified Xray binary is installed.
var XrayPath = filepath.Join(config.DefaultBinDir, "xray")

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)

// ValidateName rejects names that are unsafe in paths and unit names.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("name must be 1-31 chars of a-z, 0-9 and '-', starting with a letter or digit")
	}
	return nil
}

// ServiceName is the systemd unit (without .service) for a core.
func ServiceName(name string) string { return "rovunet-core-" + name }

// Proto is the transport-layer protocol a core listens on.
func Proto(protocol string) string {
	switch protocol {
	case "ss2022":
		return "tcp,udp"
	case "hysteria2", "wireguard", "amneziawg":
		return "udp"
	}
	return "tcp"
}

// PortFree reports whether nothing on this host already listens on port/proto.
func PortFree(port int, proto string) error { return network.PortFree(port, proto) }

// Unit builds the systemd unit for a core whose config sits in dir.
func Unit(name, dir string) *service.Unit {
	self, _ := config.SelfPath()
	condition := ""
	confirm := ""
	if _, err := os.Stat(filepath.Join(dir, optionsFile)); err == nil {
		condition = self + " core-prepare " + name
		confirm = self + " core-confirm " + name
	}
	if m, err := ReadMeta(name); err == nil && m.Protocol == "amneziawg" {
		return &service.Unit{Name: ServiceName(name), Description: "Rovunet Panel AmneziaWG: " + name, ExecStart: self + " core-awg-run " + name, ExecReload: self + " core-awg-sync " + name, ExecCondition: condition, ExecStartPost: confirm, User: "root", Group: config.SystemGroup, After: "network-online.target", Restart: "on-failure"}
	}
	return &service.Unit{
		Name:          ServiceName(name),
		Description:   "Rovunet Panel Xray core: " + name,
		ExecStart:     fmt.Sprintf("%s run -c %s", XrayForCore(name), filepath.Join(dir, "server.json")),
		ExecCondition: condition,
		ExecStartPost: confirm,
		User:          "root", // same as the VLESS service: binds privileged ports
		Group:         config.SystemGroup,
		After:         "network-online.target",
		Restart:       "always",
	}
}

// WriteBundle stores the server config, per-device client files and the
// protocol/port metadata remove needs. dir must not already exist.
func WriteBundle(dir string, o coreplan.Options, b coreplan.Bundle) (err error) {
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return fmt.Errorf("core already exists or cannot be created: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, "server.json"), b.Server, 0600); err != nil {
		return err
	}
	clients := filepath.Join(dir, "clients")
	if err := os.Mkdir(clients, 0700); err != nil {
		return err
	}
	for i, c := range b.Clients {
		if err := os.WriteFile(filepath.Join(clients, fmt.Sprintf("device-%03d.%s", i+1, b.Extension)), []byte(c+"\n"), 0600); err != nil {
			return err
		}
	}
	meta := fmt.Sprintf("protocol=%s\nport=%d\nproto=%s\n", o.Protocol, o.Port, Proto(o.Protocol))
	return os.WriteFile(filepath.Join(dir, "meta"), []byte(meta), 0600)
}

// Meta is what an installed core recorded about itself.
type Meta struct {
	Name, Protocol, Proto string
	Port                  int
}

// ReadMeta loads an installed core's metadata.
func ReadMeta(name string) (Meta, error) {
	data, err := os.ReadFile(filepath.Join(Root, name, "meta"))
	if err != nil {
		return Meta{}, err
	}
	m := Meta{Name: name}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "protocol":
			m.Protocol = v
		case "proto":
			m.Proto = v
		case "port":
			m.Port, _ = strconv.Atoi(v)
		}
	}
	if m.Protocol == "" || m.Port == 0 {
		return Meta{}, fmt.Errorf("core %q has incomplete metadata", name)
	}
	return m, nil
}

// List returns all installed cores, sorted by name.
func List() ([]Meta, error) {
	entries, err := os.ReadDir(Root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Meta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if m, err := ReadMeta(e.Name()); err == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// EnsureXray installs the Xray core published with the panel's own release,
// verified against that release's SHA256SUMS. An existing binary is kept.
func EnsureXray() error {
	if _, err := os.Stat(XrayPath); err == nil {
		return nil
	}
	url := fmt.Sprintf("%s/xray-%s-%s", binary.RuntimeDownloadBase(), runtime.GOOS, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(XrayPath), 0755); err != nil {
		return err
	}
	tmp, err := binary.Download(url, filepath.Dir(XrayPath))
	if err != nil {
		return fmt.Errorf("download xray: %w", err)
	}
	if err := os.Rename(tmp, XrayPath); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// TestConfig asks Xray itself to validate a config before it is deployed.
func TestConfig(path string) error {
	if m, err := ReadMeta(filepath.Base(filepath.Dir(path))); err == nil && m.Protocol == "amneziawg" {
		_, err := readAWG(path)
		return err
	}
	if out, err := exec.Command(XrayForCore(filepath.Base(filepath.Dir(path))), "run", "-test", "-c", path).CombinedOutput(); err != nil {
		_ = out // engine errors may contain credential material
		return fmt.Errorf("xray rejected the private configuration: %v", err)
	}
	return nil
}

// Install generates, validates and starts a new core. On any failure after
// the directory is created, everything it created is removed again.
func Install(name string, o coreplan.Options) (dir string, err error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if service.Exists(ServiceName(name)) {
		return "", fmt.Errorf("service name already exists; refusing to overwrite it")
	}
	if o.Protocol == "amneziawg" {
		if err := CheckAWGNetwork(&o); err != nil {
			return "", err
		}
	}
	proto := Proto(o.Protocol)
	if err := PortFree(o.Port, proto); err != nil {
		return "", err
	}
	b, err := coreplan.Generate(o)
	if err != nil {
		return "", err
	}
	if err := ensureEngine(o.Protocol); err != nil {
		return "", err
	}
	dir = filepath.Join(Root, name)
	if err := WriteBundle(dir, o, b); err != nil {
		return "", err
	}
	unitName := ServiceName(name)
	firewallAttempted := false
	defer func() {
		if err != nil {
			if cleanupErr := cleanupCreated(name, filepath.Join(Root, name), o.Port, proto, firewallAttempted); cleanupErr != nil {
				err = fmt.Errorf("%w; cleanup incomplete: %v", err, cleanupErr)
			}
		}
	}()
	if err = TestConfig(filepath.Join(dir, "server.json")); err != nil {
		return "", err
	}
	if err = service.Create(Unit(name, dir)); err != nil {
		return "", err
	}
	if err = service.Start(unitName); err != nil {
		return "", err
	}
	if err = waitReady(name); err != nil {
		return "", err
	}
	RecordCert(name)
	if err = EnsureCertTimer(); err != nil {
		return "", err
	}
	firewallAttempted = true
	if ferr := network.AllowOwnedPort("core:"+name, o.Port, proto); ferr != nil {
		return "", fmt.Errorf("opening %d/%s failed; installation rolled back: %w", o.Port, proto, ferr)
	}
	return dir, nil
}

// Activate (re)starts an already-written core, e.g. one restored from a
// backup: it validates the config, writes the unit, starts it and opens its
// port. A running unit is restarted so it picks up the restored config.
func Activate(name string) error {
	m, err := ReadMeta(name)
	if err != nil {
		return err
	}
	if err := ensureCoreEngine(name, m.Protocol); err != nil {
		return err
	}
	dir := filepath.Join(Root, name)
	if _, st, managed, err := loadUserCore(name); err != nil {
		return err
	} else if managed && (st.Disabled || st.Paused) {
		if err := service.Stop(ServiceName(name)); err != nil {
			return err
		}
		return service.Create(Unit(name, dir))
	}
	if err := TestConfig(filepath.Join(dir, "server.json")); err != nil {
		return err
	}
	unitName := ServiceName(name)
	if err := service.Create(Unit(name, dir)); err != nil {
		return err
	}
	if _, err := service.Status(unitName); err == nil {
		err = service.Restart(unitName)
		if err != nil {
			return err
		}
	} else if err := service.Start(unitName); err != nil {
		return err
	}
	if err := waitReady(name); err != nil {
		return err
	}
	RecordCert(name)
	if err := network.AllowOwnedPort("core:"+name, m.Port, m.Proto); err != nil {
		return err
	}
	return EnsureCertTimer()
}

// Remove stops only this core and releases only its exclusive firewall rules.
func Remove(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	m, err := ReadMeta(name)
	if err != nil {
		return fmt.Errorf("core %q not found: %w", name, err)
	}
	unitName := ServiceName(name)
	_ = collectCore(name)
	if err := service.Stop(unitName); err != nil {
		return err
	}
	if err := service.Remove(unitName); err != nil {
		return err
	}
	if err := network.ReleaseOwnedPort("core:"+name, m.Port, m.Proto); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(Root, name))
}
