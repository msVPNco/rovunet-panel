package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	iptablesRulesPath = "/etc/iptables/rovunet-rules.v4"
	// iptablesPortsPath lists the ports rovunet itself opened, one
	// "<proto> <port>" per line; the boot rules are generated from it alone.
	iptablesPortsPath = "/etc/iptables/rovunet-ports"
	iptablesUnitPath  = "/etc/systemd/system/rovunet-iptables.service"
)

// iptables-restore is invoked early at boot to re-apply rules saved by
// iptablesPersist. %s placeholders are filled with the absolute restore-binary
// path and the rules file path at write time.
const iptablesUnitTemplate = `[Unit]
Description=Restore rovunet iptables rules
DefaultDependencies=no
Before=network-pre.target
Wants=network-pre.target

[Service]
Type=oneshot
ExecStart=%s --noflush %s
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`

// HostFirewallActive reports whether the host is actually filtering inbound
// traffic. Used to decide whether to hint about external/cloud firewalls —
// merely having an iptables binary installed (e.g., pulled in by Docker or
// fail2ban) does not count if the INPUT chain still defaults to ACCEPT.
func HostFirewallActive() bool {
	if _, err := exec.LookPath("ufw"); err == nil && ufwActive() {
		return true
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil && firewalldActive() {
		return true
	}
	if _, err := exec.LookPath("iptables"); err == nil && iptablesFiltering() {
		return true
	}
	return false
}

// iptablesFiltering reports whether the INPUT chain is actually blocking
// traffic — either the default policy isn't ACCEPT, or there's at least one
// DROP/REJECT rule. ACCEPT-only chains don't filter anything.
func iptablesFiltering() bool {
	out, err := exec.Command("iptables", "-S", "INPUT").Output()
	if err != nil {
		return false
	}
	for _, line := range splitLines(string(out)) {
		if strings.HasPrefix(line, "-P INPUT ") {
			if !strings.HasSuffix(line, " ACCEPT") {
				return true
			}
			continue
		}
		if strings.Contains(line, "-j DROP") || strings.Contains(line, "-j REJECT") {
			return true
		}
	}
	return false
}

// AllowPort opens a port using the first available firewall tool.
func AllowPort(port int, proto string) error {
	// Try UFW first — but only if it's active
	if _, err := exec.LookPath("ufw"); err == nil && ufwActive() {
		return ufwAllow(port, proto)
	}

	// Try firewall-cmd — but only if it's running
	if _, err := exec.LookPath("firewall-cmd"); err == nil && firewalldActive() {
		return firewalldAllow(port, proto)
	}

	// Fallback to iptables
	if _, err := exec.LookPath("iptables"); err == nil {
		return iptablesAllow(port, proto)
	}

	// No active firewall — port is already reachable, nothing to do.
	return nil
}

// RemovePort removes a firewall rule for a port.
func RemovePort(port int, proto string) error {
	if _, err := exec.LookPath("ufw"); err == nil && ufwActive() {
		return run("ufw", "delete", "allow", fmt.Sprintf("%d/%s", port, proto))
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil && firewalldActive() {
		_ = run("firewall-cmd", "--permanent", "--remove-port", fmt.Sprintf("%d/%s", port, proto))
		return run("firewall-cmd", "--reload")
	}
	if _, err := exec.LookPath("iptables"); err == nil {
		if err := run("iptables", "-D", "INPUT", "-p", proto, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"); err != nil {
			return err
		}
		_ = trackPort(port, proto, false)
		_ = iptablesPersist()
		return nil
	}
	return nil
}

// ufwActive checks if UFW is active (not just installed).
func ufwActive() bool {
	out, err := exec.Command("ufw", "status").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Status: active")
}

// firewalldActive checks if firewalld is running.
func firewalldActive() bool {
	return exec.Command("firewall-cmd", "--state").Run() == nil
}

func ufwAllow(port int, proto string) error {
	return run("ufw", "allow", fmt.Sprintf("%d/%s", port, proto))
}

func firewalldAllow(port int, proto string) error {
	if err := run("firewall-cmd", "--permanent", "--add-port", fmt.Sprintf("%d/%s", port, proto)); err != nil {
		return err
	}
	return run("firewall-cmd", "--reload")
}

// iptablesAllow puts an ACCEPT rule for the port at the top of INPUT.
// Appending isn't enough: many images (Oracle Cloud's Ubuntu among them) end
// INPUT with a catch-all REJECT, and a rule after it never matches. An
// existing rule for the port is moved to the top, so re-running also fixes
// rules an older rovunet appended.
func iptablesAllow(port int, proto string) error {
	rule := []string{"INPUT", "-p", proto, "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT"}
	// Record the port even when the rule is already in place, so ports an
	// older rovunet opened (before it tracked them) still survive reboot.
	tracked := trackPort(port, proto, true) == nil
	if iptablesRuleFirst(rule) {
		if tracked {
			_ = iptablesPersist()
		}
		return nil
	}
	for exec.Command("iptables", append([]string{"-D"}, rule...)...).Run() == nil {
	}
	if err := run("iptables", append([]string{"-I"}, rule...)...); err != nil {
		return err
	}
	// Best-effort: persist so the rule survives reboot. The runtime rule is
	// already in effect either way, so we don't surface persistence failures.
	_ = iptablesPersist()
	return nil
}

// iptablesRuleFirst reports whether rule is already ahead of every DROP or
// REJECT rule in its chain, so it takes effect as is.
func iptablesRuleFirst(rule []string) bool {
	if exec.Command("iptables", append([]string{"-C"}, rule...)...).Run() != nil {
		return false
	}
	out, err := exec.Command("iptables", "-S", rule[0]).Output()
	if err != nil {
		return false
	}
	// iptables -S prints e.g. "-A INPUT -p tcp -m tcp --dport 443 -j ACCEPT"
	var proto, port string
	for i := 1; i+1 < len(rule); i++ {
		switch rule[i] {
		case "-p":
			proto = rule[i+1]
		case "--dport":
			port = rule[i+1]
		}
	}
	for _, line := range splitLines(string(out)) {
		f := splitFields(line)
		if len(f) < 2 || f[0] != "-A" {
			continue
		}
		if strings.Contains(line, "-j DROP") || strings.Contains(line, "-j REJECT") {
			return false
		}
		if strings.Contains(line, " -p "+proto+" ") && strings.Contains(line, "--dport "+port+" ") && strings.HasSuffix(line, "-j ACCEPT") {
			return true
		}
	}
	return false
}

// iptablesPersist writes boot rules for the ports rovunet opened and
// ensures a oneshot unit inserts them (--noflush) at boot. It used to save
// the whole iptables-save dump and replay it, which also froze other tools'
// rules (Docker's per-container NAT, fail2ban bans) and, without --noflush,
// wiped whatever had been loaded before it. Idempotent.
func iptablesPersist() error {
	restorePath, err := exec.LookPath("iptables-restore")
	if err != nil {
		return err
	}
	ports, err := readTrackedPorts(iptablesPortsPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(iptablesRulesPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(iptablesRulesPath, []byte(renderBootRules(ports)), 0644); err != nil {
		return err
	}

	unit := fmt.Sprintf(iptablesUnitTemplate, restorePath, iptablesRulesPath)
	existing, _ := os.ReadFile(iptablesUnitPath)
	if string(existing) != unit {
		if err := os.WriteFile(iptablesUnitPath, []byte(unit), 0644); err != nil {
			return err
		}
		if err := run("systemctl", "daemon-reload"); err != nil {
			return err
		}
		if err := run("systemctl", "enable", "rovunet-iptables.service"); err != nil {
			return err
		}
	}
	return nil
}

// DisableResolvedStub disables systemd-resolved's DNS stub listener on port 53
// so that rovunet's DNS router can bind to it.
func DisableResolvedStub() error {
	// Check if systemd-resolved is running
	if err := exec.Command("systemctl", "is-active", "systemd-resolved").Run(); err != nil {
		return nil // not running, nothing to do
	}

	// Create drop-in config to disable stub listener
	if err := os.MkdirAll("/etc/systemd/resolved.conf.d", 0755); err != nil {
		return fmt.Errorf("create resolved conf dir: %w", err)
	}

	conf := "[Resolve]\nDNSStubListener=no\n"
	if err := os.WriteFile("/etc/systemd/resolved.conf.d/rovunet-no-stub.conf", []byte(conf), 0644); err != nil {
		return fmt.Errorf("write resolved config: %w", err)
	}

	// Restart systemd-resolved — after this, /etc/resolv.conf (whether a
	// symlink to /run/systemd/resolve/resolv.conf or a static file
	// managed elsewhere) will list the uplink DNS servers directly
	// rather than the stub at 127.0.0.53. We intentionally do NOT write
	// /etc/resolv.conf here: on most distros it's a symlink into
	// systemd's runtime directory, so os.WriteFile follows the link and
	// the target gets regenerated on the next network event. If you want
	// public resolvers (for WARP compatibility), see
	// warp.ensurePublicResolvers which sets them via resolved drop-in.
	if err := run("systemctl", "restart", "systemd-resolved"); err != nil {
		return fmt.Errorf("restart systemd-resolved: %w", err)
	}

	return nil
}

// FreePort kills any process listening on the given port/protocol.
// Uses fuser if available, falls back to lsof + kill.
func FreePort(port int, proto string) error {
	if _, err := exec.LookPath("fuser"); err == nil {
		// fuser -k sends SIGKILL to all processes using the port
		protoFlag := fmt.Sprintf("%d/%s", port, proto)
		_ = exec.Command("fuser", "-k", protoFlag).Run()
		return nil
	}

	// Fallback: ss to find PIDs, then kill
	// Match the requested protocol: "-tlnp" alone lists only TCP listeners,
	// so freeing a UDP port used to miss the real holder and could instead
	// kill an unrelated TCP service that happened to share the port number.
	ssFlags := "-tlnp"
	if proto == "udp" {
		ssFlags = "-ulnp"
	}
	out, err := exec.Command("ss", ssFlags, fmt.Sprintf("sport = :%d", port)).Output()
	if err != nil {
		return nil // can't determine, skip
	}
	// Parse PIDs from ss output (format: pid=1234)
	for _, line := range splitLines(string(out)) {
		for _, field := range splitFields(line) {
			if len(field) > 4 && field[:4] == "pid=" {
				pid := field[4:]
				// strip trailing comma or paren
				for len(pid) > 0 && (pid[len(pid)-1] == ',' || pid[len(pid)-1] == ')') {
					pid = pid[:len(pid)-1]
				}
				_ = exec.Command("kill", "-9", pid).Run()
			}
		}
	}
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		start := i
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		if start < i {
			fields = append(fields, s[start:i])
		}
	}
	return fields
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

// PortFree reports an error if something on this host already listens on
// port/proto, by trying to bind it briefly.
func PortFree(port int, proto string) error {
	addr := net.JoinHostPort("", strconv.Itoa(port))
	if proto == "udp" {
		c, err := net.ListenPacket("udp", addr)
		if err != nil {
			return fmt.Errorf("port %d/udp is already in use", port)
		}
		return c.Close()
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d/tcp is already in use", port)
	}
	return l.Close()
}

// trackedPort is one port rovunet opened in iptables.
type trackedPort struct {
	Proto string
	Port  int
}

// readTrackedPorts parses the ports file; a missing file means none.
func readTrackedPorts(path string) ([]trackedPort, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ports []trackedPort
	seen := make(map[trackedPort]bool)
	for _, line := range splitLines(string(data)) {
		f := splitFields(line)
		if len(f) != 2 || (f[0] != "tcp" && f[0] != "udp") {
			continue
		}
		n, err := strconv.Atoi(f[1])
		if err != nil || n < 1 || n > 65535 {
			continue
		}
		p := trackedPort{f[0], n}
		if !seen[p] {
			seen[p] = true
			ports = append(ports, p)
		}
	}
	return ports, nil
}

// updateTrackedPorts returns ports with p added or removed.
func updateTrackedPorts(ports []trackedPort, p trackedPort, add bool) []trackedPort {
	out := make([]trackedPort, 0, len(ports)+1)
	for _, q := range ports {
		if q != p {
			out = append(out, q)
		}
	}
	if add {
		out = append(out, p)
	}
	return out
}

// renderBootRules is an iptables-restore file that only inserts rovunet's
// ACCEPT rules at the top of INPUT, leaving every other rule alone.
func renderBootRules(ports []trackedPort) string {
	var b strings.Builder
	b.WriteString("# Generated by rovunet: only the ports it opened.\n*filter\n")
	for _, p := range ports {
		fmt.Fprintf(&b, "-I INPUT -p %s --dport %d -j ACCEPT\n", p.Proto, p.Port)
	}
	b.WriteString("COMMIT\n")
	return b.String()
}

func trackPort(port int, proto string, add bool) error {
	ports, err := readTrackedPorts(iptablesPortsPath)
	if err != nil {
		return err
	}
	ports = updateTrackedPorts(ports, trackedPort{proto, port}, add)
	var b strings.Builder
	for _, p := range ports {
		fmt.Fprintf(&b, "%s %d\n", p.Proto, p.Port)
	}
	if err := os.MkdirAll(filepath.Dir(iptablesPortsPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(iptablesPortsPath, []byte(b.String()), 0644)
}
