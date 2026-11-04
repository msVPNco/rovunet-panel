package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSystem records commands and keeps account and group names, so the
// migration runs against a temporary /etc without touching the host.
type fakeSystem struct {
	cmds   []string
	passwd map[string]bool
	group  map[string]bool
	sshd   int
}

func setup(t *testing.T) (*fakeSystem, string) {
	t.Helper()
	root := t.TempDir()
	etcDir = filepath.Join(root, "etc")
	binDir = filepath.Join(root, "bin")
	for _, d := range []string{"slipgate/tunnels/dns", "systemd/system", "systemd/resolved.conf.d", "iptables"} {
		if err := os.MkdirAll(filepath.Join(etcDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		if err := os.WriteFile(filepath.Join(etcDir, p), []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("slipgate/config.json", `{"tunnels":[{"cert":"/etc/slipgate/tunnels/dns/server.key"}]}`)
	write("systemd/system/slipgate-dnsrouter.service", "[Unit]\nDescription=SlipGate DNS Router\n[Service]\nExecStart=/usr/local/bin/slipgate dnsrouter serve\nUser=slipgate\n")
	write("systemd/system/slipgate-socks5.service", "[Service]\nExecStart=/usr/local/bin/slipgate socks serve --creds-file /etc/slipgate/socks.creds\nUser=slipgate-socks\n")
	write("systemd/system/ssh.service", "[Service]\nExecStart=/usr/sbin/sshd\n")
	write("systemd/resolved.conf.d/slipgate-no-stub.conf", "[Resolve]\nDNSStubListener=no\n")
	write("iptables/slipgate-rules.v4", "*filter\nCOMMIT\n")

	f := &fakeSystem{
		passwd: map[string]bool{"slipgate": true, "slipgate-socks": true, "alice": true},
		group:  map[string]bool{"slipgate": true, "slipgate-socks": true, "slipgate-ssh": true},
	}
	runCmd = func(name string, args ...string) error {
		f.cmds = append(f.cmds, name+" "+strings.Join(args, " "))
		switch name {
		case "getent":
			db := f.passwd
			if args[0] == "group" {
				db = f.group
			}
			if db[args[1]] {
				return nil
			}
			return os.ErrNotExist
		case "usermod":
			delete(f.passwd, args[len(args)-1])
			f.passwd[args[1]] = true
		case "groupmod":
			delete(f.group, args[2])
			f.group[args[1]] = true
		case "systemctl":
			if args[0] == "is-active" || args[0] == "is-enabled" {
				return nil
			}
		}
		return nil
	}
	sshdFix = func() error { f.sshd++; return nil }
	return f, root
}

func TestNeeded(t *testing.T) {
	setup(t)
	if !Needed() {
		t.Fatal("a server with /etc/slipgate needs the migration")
	}
	if err := os.Mkdir(filepath.Join(etcDir, "rovunet"), 0755); err != nil {
		t.Fatal(err)
	}
	if Needed() {
		t.Fatal("never migrate over an existing /etc/rovunet")
	}
}

func TestRunMovesEverything(t *testing.T) {
	f, _ := setup(t)
	var warnings []string
	if err := Run(func(string) {}, func(s string) { warnings = append(warnings, s) }); err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(etcDir, p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read("rovunet/config.json"); !strings.Contains(got, "/etc/rovunet/tunnels/dns/server.key") || strings.Contains(got, "slipgate") {
		t.Fatalf("config paths not updated: %s", got)
	}
	if link, err := os.Readlink(filepath.Join(etcDir, "slipgate")); err != nil || link != filepath.Join(etcDir, "rovunet") {
		t.Fatalf("/etc/slipgate should link to /etc/rovunet: %q %v", link, err)
	}
	unit := read("systemd/system/rovunet-dnsrouter.service")
	for _, want := range []string{"ExecStart=/usr/local/bin/rovunet dnsrouter serve", "User=rovunet", "Description=Rovunet Panel DNS Router"} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
	if s := read("systemd/system/rovunet-socks5.service"); !strings.Contains(s, "User=rovunet-socks") || !strings.Contains(s, "/etc/rovunet/socks.creds") {
		t.Fatalf("socks unit not rewritten:\n%s", s)
	}
	for _, gone := range []string{"systemd/system/slipgate-dnsrouter.service", "systemd/system/slipgate-socks5.service", "systemd/resolved.conf.d/slipgate-no-stub.conf", "iptables/slipgate-rules.v4"} {
		if _, err := os.Stat(filepath.Join(etcDir, gone)); err == nil {
			t.Fatalf("%s should have been renamed", gone)
		}
	}
	for _, want := range []string{"systemd/resolved.conf.d/rovunet-no-stub.conf", "iptables/rovunet-rules.v4", "systemd/system/ssh.service", "rovunet/" + doneMarker} {
		if _, err := os.Stat(filepath.Join(etcDir, want)); err != nil {
			t.Fatalf("missing %s", want)
		}
	}
	if !f.passwd["rovunet"] || !f.passwd["rovunet-socks"] || f.passwd["slipgate"] || !f.passwd["alice"] {
		t.Fatalf("accounts: %v", f.passwd)
	}
	if !f.group["rovunet-ssh"] || f.group["slipgate-ssh"] {
		t.Fatalf("groups: %v", f.group)
	}
	if f.sshd != 1 {
		t.Fatal("sshd_config block must be renamed once the group is")
	}
	joined := strings.Join(f.cmds, "\n")
	for _, want := range []string{"systemctl stop slipgate-dnsrouter", "systemctl enable rovunet-dnsrouter", "systemctl start rovunet-socks5"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing command %q in:\n%s", want, joined)
		}
	}
	if link, err := os.Readlink(filepath.Join(binDir, "slipgate")); err != nil || link != filepath.Join(binDir, "rovunet") {
		t.Fatalf("slipgate should be a link to rovunet: %q %v", link, err)
	}

	// A second run finds nothing to do
	f.cmds = nil
	if err := Run(func(string) {}, func(string) {}); err != nil || len(f.cmds) != 0 {
		t.Fatalf("second run should be a no-op: %v %v", err, f.cmds)
	}
}

func TestRunLeavesSSHDWhenGroupRenameFails(t *testing.T) {
	f, _ := setup(t)
	inner := runCmd
	runCmd = func(name string, args ...string) error {
		if name == "groupmod" && args[len(args)-1] == "slipgate-ssh" {
			return os.ErrPermission
		}
		return inner(name, args...)
	}
	if err := Run(func(string) {}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if f.sshd != 0 {
		t.Fatal("sshd_config must keep the old group name when the group wasn't renamed")
	}
}
