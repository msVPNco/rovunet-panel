// Package migrate moves a server installed under the old SlipGate names
// (slipgate binary, /etc/slipgate, slipgate-* services and accounts) to the
// Rovunet names, keeping every tunnel, key, user and setting.
package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/system"
)

const (
	legacyName   = "slipgate"
	legacyPrefix = "slipgate-"
	newPrefix    = "rovunet-"
	doneMarker   = ".migrated-from-slipgate"
)

// Paths and commands the migration uses; tests replace them.
var (
	etcDir  = "/etc"
	binDir  = config.DefaultBinDir
	runCmd  = func(name string, args ...string) error { return exec.Command(name, args...).Run() }
	sshdFix = system.MigrateLegacySSHConfig
)

// Needed reports whether this server still runs under the old names.
func Needed() bool {
	fi, err := os.Lstat(filepath.Join(etcDir, "slipgate"))
	if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(etcDir, "rovunet"))
	return os.IsNotExist(err)
}

// Run migrates the server. It is safe to run again: every step checks
// whether it is still needed. Problems that leave the server working are
// reported through warn and do not stop the migration.
func Run(info, warn func(string)) error {
	if !Needed() {
		return nil
	}
	info("Moving this server from SlipGate names to Rovunet names...")

	units := legacyUnits()
	states := map[string]unitState{}
	for _, u := range units {
		states[u] = currentState(u)
		_ = runCmd("systemctl", "stop", u)
	}

	// Accounts first: unit files and sshd_config refer to them by name.
	sshGroupRenamed := renameAccounts(warn)

	oldDir := filepath.Join(etcDir, "slipgate")
	newDir := filepath.Join(etcDir, "rovunet")
	if err := os.Rename(oldDir, newDir); err != nil {
		// Nothing has moved yet: start the services again as they were.
		for _, u := range units {
			if states[u].active {
				_ = runCmd("systemctl", "start", u)
			}
		}
		return fmt.Errorf("move %s to %s: %w", oldDir, newDir, err)
	}
	// Paths written into files we don't regenerate keep working.
	if err := os.Symlink(newDir, oldDir); err != nil {
		warn("Could not link " + oldDir + ": " + err.Error())
	}
	if err := rewriteFile(filepath.Join(newDir, "config.json")); err != nil {
		warn("Could not update paths in config.json: " + err.Error())
	}

	renamePrefixed(filepath.Join(etcDir, "iptables"), "slipgate-rules", warn)
	renamePrefixed(filepath.Join(etcDir, "systemd", "resolved.conf.d"), legacyPrefix, warn)

	for _, u := range units {
		if err := moveUnit(u); err != nil {
			warn(fmt.Sprintf("Could not rename %s: %v", u, err))
		}
	}
	_ = runCmd("systemctl", "daemon-reload")
	for _, u := range units {
		n := newUnitName(u)
		if states[u].enabled {
			_ = runCmd("systemctl", "enable", n)
		}
		if states[u].active {
			if err := runCmd("systemctl", "start", n); err != nil {
				warn(fmt.Sprintf("Could not start %s: %v", n, err))
			}
		}
	}

	if sshGroupRenamed {
		if err := sshdFix(); err != nil {
			warn("SSH settings were left as they were: " + err.Error())
		}
	}

	if err := installCommand(); err != nil {
		warn("Could not install the rovunet command: " + err.Error())
	}

	_ = os.WriteFile(filepath.Join(newDir, doneMarker), []byte("migrated\n"), 0600)
	info("Done: the command is now 'rovunet' ('slipgate' still works for now).")
	return nil
}

type unitState struct{ active, enabled bool }

func currentState(unit string) unitState {
	return unitState{
		active:  runCmd("systemctl", "is-active", "--quiet", unit) == nil,
		enabled: runCmd("systemctl", "is-enabled", "--quiet", unit) == nil,
	}
}

// legacyUnits lists slipgate-* service files, without the suffix.
func legacyUnits() []string {
	entries, err := os.ReadDir(filepath.Join(etcDir, "systemd", "system"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, legacyPrefix) && strings.HasSuffix(n, ".service") && e.Type().IsRegular() {
			names = append(names, strings.TrimSuffix(n, ".service"))
		}
	}
	return names
}

func newUnitName(old string) string {
	return newPrefix + strings.TrimPrefix(old, legacyPrefix)
}

// moveUnit writes the unit under its new name with the new paths and
// accounts, then removes the old one.
func moveUnit(old string) error {
	dir := filepath.Join(etcDir, "systemd", "system")
	oldPath := filepath.Join(dir, old+".service")
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return err
	}
	newPath := filepath.Join(dir, newUnitName(old)+".service")
	if err := os.WriteFile(newPath, []byte(RewriteText(string(data))), 0644); err != nil {
		return err
	}
	_ = runCmd("systemctl", "disable", old)
	return os.Remove(oldPath)
}

// RewriteText changes the old names in a unit file or config: paths,
// service, account and command names. HMAC labels and upstream download
// URLs never appear in these files.
func RewriteText(s string) string {
	return strings.NewReplacer(
		"/etc/slipgate", "/etc/rovunet",
		"/usr/local/bin/slipgate", "/usr/local/bin/rovunet",
		"slipgate-", "rovunet-",
		"User=slipgate", "User=rovunet",
		"Group=slipgate", "Group=rovunet",
		"Description=SlipGate", "Description=Rovunet Panel",
	).Replace(s)
}

func rewriteFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := strings.ReplaceAll(string(data), "/etc/slipgate/", "/etc/rovunet/")
	if out == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(out), 0600)
}

// renamePrefixed renames dir/<prefix>... files to the new prefix.
func renamePrefixed(dir, prefix string, warn func(string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		n := strings.Replace(e.Name(), legacyName, "rovunet", 1)
		if err := os.Rename(filepath.Join(dir, e.Name()), filepath.Join(dir, n)); err != nil {
			warn(fmt.Sprintf("Could not rename %s: %v", e.Name(), err))
		}
	}
}

// renameAccounts renames the panel's system accounts and groups. It reports
// whether the SSH tunnel users' group now has its new name.
func renameAccounts(warn func(string)) bool {
	for _, a := range []struct{ old, new string }{
		{"slipgate", config.SystemUser},
		{"slipgate-socks", "rovunet-socks"},
		{"slipgate-naive", "rovunet-naive"},
	} {
		if exists("passwd", a.old) && !exists("passwd", a.new) {
			args := []string{"-l", a.new, "-d", "/home/" + a.new}
			if fi, err := os.Stat("/home/" + a.old); err == nil && fi.IsDir() {
				args = append(args, "-m") // move an existing home along
			}
			if err := runCmd("usermod", append(args, a.old)...); err != nil {
				warn(fmt.Sprintf("Could not rename account %s: %v", a.old, err))
			}
		}
		if exists("group", a.old) && !exists("group", a.new) {
			if err := runCmd("groupmod", "-n", a.new, a.old); err != nil {
				warn(fmt.Sprintf("Could not rename group %s: %v", a.old, err))
			}
		}
	}
	if exists("group", "slipgate-ssh") && !exists("group", config.SSHGroup) {
		if err := runCmd("groupmod", "-n", config.SSHGroup, "slipgate-ssh"); err != nil {
			warn("Could not rename group slipgate-ssh: " + err.Error())
			return false
		}
	}
	return exists("group", config.SSHGroup)
}

// exists reports whether name is in /etc/passwd or /etc/group.
func exists(db, name string) bool {
	return runCmd("getent", db, name) == nil
}

// installCommand puts the binary at /usr/local/bin/rovunet and keeps
// /usr/local/bin/slipgate as a link to it.
func installCommand() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, _ = filepath.EvalSymlinks(self)
	target := filepath.Join(binDir, "rovunet")
	legacy := filepath.Join(binDir, legacyName)
	if self != target {
		data, err := os.ReadFile(self)
		if err != nil {
			return err
		}
		tmp := target + ".new"
		if err := os.WriteFile(tmp, data, 0755); err != nil {
			return err
		}
		if err := os.Rename(tmp, target); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	if fi, err := os.Lstat(legacy); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	_ = os.Remove(legacy) // the old binary; renaming over a running file is fine
	return os.Symlink(target, legacy)
}
