package coreinstall

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

var AWGGoPath = filepath.Join(config.DefaultBinDir, "rovunet-amneziawg-go")
var AWGToolPath = filepath.Join(config.DefaultBinDir, "rovunet-awg")

const AWGGoCommit = "b5928efb6ca19f0153958460c3d141f04abc5c2e"
const AWGToolsCommit = "ee0f0a9aa34ff0a0da4b3433b9512781cfe02843"

func AWGInterface(name string) string {
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("ra%x", sum[:6])
}

func ensureEngine(protocol string) error {
	if protocol != "amneziawg" {
		if err := EnsureXray(); err != nil {
			return err
		}
		if liveUserProtocol(protocol) {
			return verifyXray()
		}
		return nil
	}
	for _, item := range []struct{ name, path string }{{"amneziawg-go", AWGGoPath}, {"awg", AWGToolPath}} {
		if _, err := os.Stat(item.path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(item.path), 0755); err != nil {
			return err
		}
		// binary.Download verifies the asset against this release's SHA256SUMS.
		tmp, err := binary.Download(fmt.Sprintf("%s/%s-linux-%s", binary.RuntimeDownloadBase(), item.name, runtime.GOARCH), filepath.Dir(item.path))
		if err != nil {
			return fmt.Errorf("download verified %s: %w", item.name, err)
		}
		if err := os.Rename(tmp, item.path); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	if _, err := os.Stat(awgManifestPath()); os.IsNotExist(err) {
		tmp, err := binary.Download(fmt.Sprintf("%s/awg-provenance-linux-%s.json", binary.RuntimeDownloadBase(), runtime.GOARCH), filepath.Dir(AWGGoPath))
		if err != nil {
			return err
		}
		if err := os.Rename(tmp, awgManifestPath()); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return recordAWG()
}

func readAWG(path string) (coreplan.AWGServer, error) {
	var s coreplan.AWGServer
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	validKey := func(k string) bool { v, e := base64.StdEncoding.DecodeString(k); return e == nil && len(v) == 32 }
	if !validKey(s.PrivateKey) || !validKey(s.Obfuscation.HeaderProtectionKey) || s.Port < 1 || s.Port > 65535 {
		return s, fmt.Errorf("invalid AmneziaWG keys or port")
	}
	o := coreplan.Options{Protocol: "amneziawg", Host: "127.0.0.1", Port: s.Port, Subnet: s.Subnet, Egress: s.Egress}
	if err := coreplan.Validate(&o); err != nil {
		return s, err
	}
	_, block, _ := net.ParseCIDR(s.Subnet)
	seenKeys, seenIPs := map[string]bool{}, map[string]bool{}
	for _, p := range s.Peers {
		if config.ValidateUsername(p.Name) != nil || !validKey(p.PublicKey) || !block.Contains(net.ParseIP(p.Address)) || seenKeys[p.PublicKey] || seenIPs[p.Address] {
			return s, fmt.Errorf("invalid AmneziaWG peer")
		}
		seenKeys[p.PublicKey], seenIPs[p.Address] = true, true
	}
	return s, nil
}

func awgConfigFile(name string) (string, error) {
	dir := filepath.Join(Root, name)
	s, err := readAWG(filepath.Join(dir, "server.json"))
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".awg-*.conf")
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(s.SetConf())
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// SyncAWG replaces only the changed peers; healthy peers keep their sessions.
func SyncAWG(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	path, err := awgConfigFile(name)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	if err := exec.Command(AWGToolPath, "syncconf", AWGInterface(name), path).Run(); err != nil {
		return fmt.Errorf("AmneziaWG peer update rejected: %w", err)
	}
	return nil
}

func DefaultEgress() (string, error) {
	out, err := exec.Command("ip", "-j", "route", "get", "1.1.1.1").Output()
	if err != nil {
		return "", err
	}
	var routes []struct {
		Dev string `json:"dev"`
	}
	if json.Unmarshal(out, &routes) != nil || len(routes) == 0 || routes[0].Dev == "" {
		return "", fmt.Errorf("cannot determine Internet egress interface")
	}
	return routes[0].Dev, nil
}

// CheckAWGNetwork rejects a subnet already routed on this host. The test
// interface and its keys are deliberately never adopted or overwritten.
func CheckAWGNetwork(o *coreplan.Options) error {
	if o.Egress == "" {
		v, err := DefaultEgress()
		if err != nil {
			return err
		}
		o.Egress = v
	}
	if err := coreplan.Validate(o); err != nil {
		return err
	}
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return fmt.Errorf("AmneziaWG requires /dev/net/tun")
	}
	out, err := exec.Command("ip", "-j", "route", "show", "table", "all").Output()
	if err != nil {
		return err
	}
	var routes []struct {
		Dst string `json:"dst"`
	}
	if err := json.Unmarshal(out, &routes); err != nil {
		return err
	}
	_, want, _ := net.ParseCIDR(o.Subnet)
	for _, r := range routes {
		if r.Dst == "default" {
			continue
		}
		ip, have, err := net.ParseCIDR(r.Dst)
		if err != nil {
			ip = net.ParseIP(r.Dst)
			if ip != nil && want.Contains(ip) {
				return fmt.Errorf("AmneziaWG subnet overlaps an existing host route")
			}
			continue
		}
		if want.Contains(ip) || have.Contains(want.IP) {
			return fmt.Errorf("AmneziaWG subnet overlaps an existing route")
		}
	}
	return nil
}

func awgRules(name string, s coreplan.AWGServer) [][]string {
	iface, comment := AWGInterface(name), "rovunet-awg-"+name
	return [][]string{
		{"-t", "nat", "POSTROUTING", "-s", s.Subnet, "-o", s.Egress, "-m", "comment", "--comment", comment, "-j", "MASQUERADE"},
		{"-t", "filter", "FORWARD", "-i", iface, "-o", s.Egress, "-m", "comment", "--comment", comment, "-j", "ACCEPT"},
		{"-t", "filter", "FORWARD", "-i", s.Egress, "-o", iface, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-m", "comment", "--comment", comment, "-j", "ACCEPT"},
	}
}

func setAWGRules(name string, s coreplan.AWGServer, add bool) error {
	for _, rule := range awgRules(name, s) {
		base := rule[:2]
		tail := rule[2:]
		has := exec.Command("iptables", append(append(append([]string{}, base...), "-C"), tail...)...).Run() == nil
		if add && has || !add && !has {
			continue
		}
		op := "-D"
		if add {
			op = "-I"
		}
		args := append(append(append([]string{}, base...), op), tail...)
		if err := exec.Command("iptables", args...).Run(); err != nil {
			return fmt.Errorf("AmneziaWG scoped firewall rule: %w", err)
		}
	}
	return nil
}

// RunAWG supervises the official userspace engine. Network setup is scoped to
// its interface and commented rules; no kernel module or Docker is required.
func RunAWG(ctx context.Context, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	s, err := readAWG(filepath.Join(Root, name, "server.json"))
	if err != nil {
		return err
	}
	if s.Egress == "" {
		return fmt.Errorf("AmneziaWG egress interface is required")
	}
	iface := AWGInterface(name)
	if exec.Command("ip", "link", "show", "dev", iface).Run() == nil {
		return fmt.Errorf("AmneziaWG interface already exists; refusing to replace it")
	}
	cmd := exec.Command(AWGGoPath, "-f", iface)
	cmd.Env = append(os.Environ(), "LOG_LEVEL=error")
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		_ = setAWGRules(name, s, false)
	}()
	ready := false
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); {
		if exec.Command(AWGToolPath, "show", iface, "listen-port").Run() == nil {
			ready = true
			break
		}
		select {
		case err := <-done:
			return fmt.Errorf("AmneziaWG engine exited: %w", err)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		return fmt.Errorf("AmneziaWG UAPI did not become ready")
	}
	path, err := awgConfigFile(name)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	if err := exec.Command(AWGToolPath, "setconf", iface, path).Run(); err != nil {
		return fmt.Errorf("AmneziaWG rejected configuration: %w", err)
	}
	for _, args := range [][]string{{"address", "add", s.Address, "dev", iface}, {"link", "set", "dev", iface, "mtu", "1280", "up"}} {
		if err := exec.Command("ip", args...).Run(); err != nil {
			return err
		}
	}
	if err := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run(); err != nil {
		return err
	}
	if err := setAWGRules(name, s, true); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-done:
		return fmt.Errorf("AmneziaWG engine stopped: %w", err)
	}
}

func awgListening(name string, port int) bool {
	b, err := exec.Command(AWGToolPath, "show", AWGInterface(name), "listen-port").Output()
	return err == nil && strings.TrimSpace(string(b)) == strconv.Itoa(port)
}
