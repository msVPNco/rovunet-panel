package system

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

// EnsureUser creates the rovunet system user and groups if they don't exist.
func EnsureUser() error {
	// Create group
	if !groupExists(config.SystemGroup) {
		if err := run("groupadd", "--system", config.SystemGroup); err != nil {
			return fmt.Errorf("create group %s: %w", config.SystemGroup, err)
		}
	}

	// Create SSH group
	if !groupExists(config.SSHGroup) {
		if err := run("groupadd", "--system", config.SSHGroup); err != nil {
			return fmt.Errorf("create group %s: %w", config.SSHGroup, err)
		}
	}

	// Create user
	if !userExists(config.SystemUser) {
		if err := run("useradd", "--system", "--no-create-home",
			"--shell", "/usr/sbin/nologin",
			"--gid", config.SystemGroup,
			config.SystemUser); err != nil {
			return fmt.Errorf("create user %s: %w", config.SystemUser, err)
		}
	}

	return nil
}

// RemoveUser removes the rovunet system user and groups.
func RemoveUser() error {
	// SSH users must go first: rovunet-ssh is their primary group, so
	// groupdel refuses while any remain, and they'd otherwise outlive the
	// uninstall as password-enabled accounts.
	if users, err := ListSSHUsers(); err == nil {
		for _, u := range users {
			_ = RemoveSSHUser(u)
		}
	}
	_ = run("userdel", config.SystemUser)
	_ = run("groupdel", config.SSHGroup)
	_ = run("groupdel", config.SystemGroup)
	return nil
}

// RemoveSpecificUser removes a system user by name.
func RemoveSpecificUser(username string) error {
	return run("userdel", username)
}

// EnsureDir changes only the requested directory. A configuration tree may
// also contain root-only backup identities, core keys and recovery snapshots;
// recursively changing their ownership grants the service account access.
func EnsureDir(path, owner string) error {
	if err := os.MkdirAll(path, 0750); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed configuration directory must be a regular directory")
	}
	// The service group must traverse this directory even under umask 077.
	if err := os.Chmod(path, 0750); err != nil {
		return err
	}
	return run("chown", "--no-dereference", "--", owner+":"+config.SystemGroup, path)
}

// AddSSHUser creates a system user in the rovunet-ssh group.
// If the user already exists, it updates the password instead.
func AddSSHUser(username, password string) error {
	if err := config.ValidateUsername(username); err != nil {
		return err
	}
	if err := config.ValidatePassword(password); err != nil {
		return err
	}
	if userExists(username) {
		if err := requireOwnedSSHAccount(username); err != nil {
			return err
		}
	}
	// Ensure SSH group exists
	if !groupExists(config.SSHGroup) {
		if err := run("groupadd", "--system", config.SSHGroup); err != nil {
			return fmt.Errorf("create group %s: %w", config.SSHGroup, err)
		}
	}

	if !userExists(username) {
		if err := run("useradd", "--system", "--no-create-home",
			"--shell", "/bin/false",
			"--gid", config.SSHGroup,
			username); err != nil {
			return fmt.Errorf("create user: %w", err)
		}
	}

	// Set password
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s", username, password))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	// Ensure sshd Match Group config
	return ensureSSHMatchGroup()
}

// RemoveSSHUser kills active sessions and removes a user from the system.
func RemoveSSHUser(username string) error {
	if err := requireOwnedSSHAccount(username); err != nil {
		return err
	}
	// SIGKILL all processes owned by the user so userdel isn't blocked by a
	// live SSH session. pkill returns before the kernel reaps the processes,
	// so userdel -f forces removal even if a process is briefly still listed.
	if err := TerminateSSHUserSessions(username); err != nil {
		return fmt.Errorf("could not end owned SSH sessions; account retained: %w", err)
	}
	return run("userdel", "-f", username)
}

// TerminateSSHUserSessions is called only after an explicit password change,
// never by routine startup/update credential synchronization.
func TerminateSSHUserSessions(username string) error {
	if err := requireOwnedSSHAccount(username); err != nil {
		return err
	}
	err := exec.Command("pkill", "-KILL", "-u", username).Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil // no matching session
	}
	return err
}

func requireOwnedSSHAccount(username string) error {
	if err := config.ValidateUsername(username); err != nil {
		return err
	}
	uidBytes, err := exec.Command("id", "-u", username).Output()
	uid, parseErr := strconv.ParseUint(strings.TrimSpace(string(uidBytes)), 10, 32)
	if err != nil || parseErr != nil || uid == 0 || username == config.SystemUser || !IsSSHUser(username) {
		return errors.New("refusing to modify an account outside the owned Rovunet SSH group")
	}
	return nil
}

// ListSSHUsers returns all users in the rovunet-ssh group: those whose
// primary group it is (how AddSSHUser creates them, which /etc/group's
// member list never shows) as well as any explicit supplementary members.
func ListSSHUsers() ([]string, error) {
	return listGroupUsers("/etc/group", "/etc/passwd", config.SSHGroup)
}

func listGroupUsers(groupFile, passwdFile, group string) ([]string, error) {
	gdata, err := os.ReadFile(groupFile)
	if err != nil {
		return nil, err
	}
	gid := ""
	var users []string
	seen := make(map[string]bool)
	add := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			users = append(users, u)
		}
	}
	for _, line := range strings.Split(string(gdata), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 4 && parts[0] == group {
			gid = parts[2]
			for _, m := range strings.Split(parts[3], ",") {
				add(strings.TrimSpace(m))
			}
			break
		}
	}
	if gid == "" {
		return nil, nil // group doesn't exist
	}
	pdata, err := os.ReadFile(passwdFile)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(pdata), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 4 && parts[3] == gid {
			add(parts[0])
		}
	}
	return users, nil
}

// GeneratePassword generates a random alphanumeric password.
func GeneratePassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return string(result)
}

const sshGlobalSettings = `
# Rovunet global SSH settings
MaxStartups 100:30:300
MaxSessions 100
TCPKeepAlive no
`

// sshMatchBlock configures the tunnel users. They log in with passwords,
// which many images turn off globally (e.g. cloud-init's
// PasswordAuthentication no, or AuthenticationMethods publickey), so the
// block turns them back on for this group alone; other accounts keep the
// server's own rules.
const sshMatchBlock = `
# Rovunet SSH tunnel users
Match Group rovunet-ssh
    PasswordAuthentication yes
    AuthenticationMethods any
    AllowTcpForwarding local
    AllowStreamLocalForwarding no
    GatewayPorts no
    PermitTunnel no
    PermitTTY no
    PermitUserRC no
    X11Forwarding no
    AllowAgentForwarding no
    ForceCommand /bin/false
    ClientAliveInterval 30
    ClientAliveCountMax 10
`

// EnsureSSHConfig brings sshd_config's Rovunet settings up to date, e.g.
// after an update changes them.
func EnsureSSHConfig() error {
	return ensureSSHMatchGroup()
}

// withRovunetSSHConfig returns sshd_config content with Rovunet's global
// settings and its current Match block.
func withRovunetSSHConfig(content string) string {
	content = renameLegacySSHConfig(content)
	// Global settings must come before any Match block
	if !strings.Contains(content, "# Rovunet global SSH settings") {
		// Insert before the first Match block or Rovunet block, or append
		if idx := strings.Index(content, "# Rovunet SSH tunnel users"); idx >= 0 {
			content = content[:idx] + sshGlobalSettings + "\n" + content[idx:]
		} else if idx := strings.Index(content, "\nMatch "); idx >= 0 {
			content = content[:idx] + sshGlobalSettings + content[idx:]
		} else {
			content += sshGlobalSettings
		}
	}

	if strings.Contains(content, sshMatchBlock) {
		return content // already up to date
	}
	// Replace an older Rovunet block, which is always last in the file
	if idx := strings.Index(content, "# Rovunet SSH tunnel users"); idx >= 0 {
		return replaceOwnedSSHBlock(content, idx)
	} else if strings.Contains(content, "Match Group rovunet-ssh") {
		return content // a block we didn't write: can't safely replace it
	}
	return content + sshMatchBlock
}

func ensureSSHMatchGroup() error {
	const path = "/etc/ssh/sshd_config"
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := withRovunetSSHConfig(string(data))
	if content == string(data) {
		return nil
	}

	// Validate before replacing the live config: an sshd_config that fails
	// to parse stops sshd from starting and locks out the server.
	tmp := path + ".rovunet-new"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	// sshd -t refuses to run without its privilege separation directory.
	// systemd creates it only while ssh.service runs, and on Ubuntu 22.10+
	// sshd is started on demand by ssh.socket, so it is often missing.
	if err := ensureSSHDRunDir(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := run("sshd", "-t", "-f", tmp); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("sshd config validation failed: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	// Try reload first, fall back to restart (needed on Ubuntu 24.04+ with socket activation)
	for _, svc := range []string{"sshd", "ssh"} {
		if exec.Command("systemctl", "is-active", svc+".service").Run() == nil {
			if err := run("systemctl", "reload", svc); err == nil {
				return nil
			}
			return run("systemctl", "restart", svc)
		}
	}
	return nil
}

// sshdRunDir is OpenSSH's privilege separation directory on Debian/Ubuntu.
var sshdRunDir = "/run/sshd"

// ensureSSHDRunDir creates the privilege separation directory the way the
// ssh.service unit would (RuntimeDirectory=sshd, mode 0755).
func ensureSSHDRunDir() error {
	if _, err := os.Stat(sshdRunDir); err == nil {
		return nil
	}
	if err := os.MkdirAll(sshdRunDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", sshdRunDir, err)
	}
	return os.Chmod(sshdRunDir, 0755)
}

// UserExists reports whether a system account with this name exists.
func UserExists(name string) bool {
	return userExists(name)
}

// IsSSHUser reports whether name is an account in the rovunet-ssh group.
// AddSSHUser makes that group the primary one, which /etc/group's member
// list doesn't show, so this asks id instead.
func IsSSHUser(name string) bool {
	out, err := exec.Command("id", "-Gn", name).Output()
	if err != nil {
		return false
	}
	for _, g := range strings.Fields(string(out)) {
		if g == config.SSHGroup {
			return true
		}
	}
	return false
}

func userExists(name string) bool {
	err := exec.Command("id", name).Run()
	return err == nil
}

func groupExists(name string) bool {
	err := exec.Command("getent", "group", name).Run()
	return err == nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w (%s)", name, args, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// legacySSHNames maps the markers and group a pre-rename (SlipGate-era)
// install wrote into sshd_config to their current names, so its blocks are
// recognized and kept up to date instead of being left behind beside new ones.
var legacySSHNames = []struct{ old, new string }{
	{"# Slip" + "Gate global SSH settings", "# Rovunet global SSH settings"},
	{"# Slip" + "Gate SSH tunnel users", "# Rovunet SSH tunnel users"},
	{"Match Group slip" + "gate-ssh", "Match Group " + config.SSHGroup},
}

func renameLegacySSHConfig(content string) string {
	for _, n := range legacySSHNames {
		content = strings.ReplaceAll(content, n.old, n.new)
	}
	return content
}
