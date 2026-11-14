package system

import (
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func withoutOwnedSSHConfig(content string) (string, error) {
	// Legacy markers are normalized only in memory; an unknown block is
	// preserved rather than treated as a license to rewrite administrator rules.
	content = renameLegacySSHConfig(content)
	if start := strings.Index(content, sshMatchBlock); start >= 0 {
		tail := content[start+len(sshMatchBlock):]
		for _, line := range strings.Split(tail, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
				continue
			}
			if strings.EqualFold(fields[0], "Match") {
				break
			}
			return "", errors.New("custom directives follow the owned SSH Match block; retained for review")
		}
	}
	for _, block := range []string{sshGlobalSettings, sshMatchBlock} {
		content = strings.ReplaceAll(content, block, "")
		content = strings.ReplaceAll(content, strings.TrimPrefix(block, "\n"), "")
	}
	if strings.Contains(content, "# Rovunet") || strings.Contains(content, "Match Group rovunet-ssh") {
		return "", errors.New("custom or older SSH block needs review; SSH configuration retained")
	}
	return content, nil
}

// RemoveOwnedSSHConfig validates the candidate before replacement and reloads
// only an active SSH service. It never restarts a login daemon as a fallback.
func RemoveOwnedSSHConfig() error {
	const path = "/etc/ssh/sshd_config"
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	clean, err := withoutOwnedSSHConfig(string(raw))
	if err != nil || clean == string(raw) {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".rovunet-uninstall-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.WriteString(clean); err == nil {
		err = f.Chmod(info.Mode().Perm())
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	if err = ensureSSHDRunDir(); err != nil {
		return err
	}
	if err = run("sshd", "-t", "-f", f.Name()); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	for _, unit := range []string{"ssh.service", "sshd.service"} {
		if exec.Command("systemctl", "is-active", "--quiet", unit).Run() == nil {
			return run("systemctl", "reload", unit)
		}
	}
	return nil
}

// Legacy tunnel accounts are removed only if their primary and sole group is
// the managed SSH group and their shell is /bin/false. A sudo/admin account is
// never adopted, password-reset or removed by this cleanup.
func RemoveOwnedTunnelAccounts() error {
	for _, group := range []string{"rovunet-ssh", "slipgate-ssh"} {
		users, err := listGroupUsers("/etc/group", "/etc/passwd", group)
		if err != nil {
			return err
		}
		for _, user := range users {
			if err := config.ValidateUsername(user); err != nil {
				return err
			}
			uid, e1 := exec.Command("id", "-u", user).Output()
			groups, e2 := exec.Command("id", "-Gn", user).Output()
			primary, e3 := exec.Command("id", "-gn", user).Output()
			entry, e4 := exec.Command("getent", "passwd", user).Output()
			id, e5 := strconv.ParseUint(strings.TrimSpace(string(uid)), 10, 32)
			fields := strings.Split(strings.TrimSpace(string(entry)), ":")
			if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || id == 0 || len(fields) != 7 || fields[6] != "/bin/false" || strings.TrimSpace(string(groups)) != group || strings.TrimSpace(string(primary)) != group {
				return fmt.Errorf("account %q has non-tunnel permissions; retained for explicit review", user)
			}
			p := exec.Command("pkill", "-KILL", "-u", user).Run()
			var exit *exec.ExitError
			if p != nil && !(errors.As(p, &exit) && exit.ExitCode() == 1) {
				return p
			}
			if err := run("userdel", user); err != nil {
				return err
			}
		}
		if groupExists(group) {
			if err := run("groupdel", group); err != nil {
				return err
			}
		}
	}
	return nil
}

func RemovePanelServiceAccounts() error {
	for _, name := range []string{"rovunet", "slipgate"} {
		if userExists(name) {
			entry, err := exec.Command("getent", "passwd", name).Output()
			fields := strings.Split(strings.TrimSpace(string(entry)), ":")
			groups, groupErr := exec.Command("id", "-Gn", name).Output()
			if err != nil || groupErr != nil || len(fields) != 7 || fields[2] == "0" || (fields[6] != "/usr/sbin/nologin" && fields[6] != "/sbin/nologin") || strings.TrimSpace(string(groups)) != name {
				return fmt.Errorf("service account %q has custom privileges; retained for review", name)
			}
			if err := run("userdel", name); err != nil {
				return err
			}
		}
		if groupExists(name) {
			if err := run("groupdel", name); err != nil {
				return err
			}
		}
	}
	return nil
}
