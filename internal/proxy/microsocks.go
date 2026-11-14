package proxy

import (
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/system"
)

const (
	socksServiceName = "rovunet-socks5"
	// socksCredsFile holds `user:pass` lines, one per user. The `socks serve`
	// command reads this at start and on SIGHUP, so credential changes do not
	// require a process restart (which would drop every active client).
	socksCredsFile = config.DefaultConfigDir + "/socks-users.conf"
)

// RunAsUser overrides the system user for the SOCKS5 service.
// When non-empty the service runs as this user instead of config.SystemUser.
// Set this before calling any Setup* function when WARP routing is active.
var RunAsUser string

// SetupSOCKS creates the SOCKS5 proxy service (localhost only, no auth).
func SetupSOCKS() error {
	return setupSOCKSMulti("127.0.0.1", nil, false)
}

// SetupSOCKSWithAuth creates the SOCKS5 proxy with a single user (localhost only).
func SetupSOCKSWithAuth(user, password string) error {
	return setupSOCKSMulti("127.0.0.1", []config.UserConfig{{Username: user, Password: password}}, false)
}

// SetupSOCKSWithUsers creates the SOCKS5 proxy with multiple users (localhost only).
func SetupSOCKSWithUsers(users []config.UserConfig) error {
	return setupSOCKSMulti("127.0.0.1", users, false)
}

// SetupSOCKSExternal creates the SOCKS5 proxy on all interfaces (for direct SOCKS5).
// requireAuth is always on for this listener: a non-loopback bind must never
// fall back to an open relay.
func SetupSOCKSExternal(user, password string) error {
	return setupSOCKSMulti("0.0.0.0", []config.UserConfig{{Username: user, Password: password}}, true)
}

// SetupSOCKSExternalWithUsers creates the SOCKS5 proxy on all interfaces with
// multiple users. requireAuth is always on: if users is empty (e.g. the last
// user was just removed), the resulting listener rejects every connection
// instead of serving as an unauthenticated open relay on 0.0.0.0. This is the
// shared enforcement point for every caller of this function — a caller that
// forgets to guard against an empty user list is still safe.
func SetupSOCKSExternalWithUsers(users []config.UserConfig) error {
	return setupSOCKSMulti("0.0.0.0", users, true)
}

func setupSOCKSMulti(listenAddr string, users []config.UserConfig, requireAuth bool) error {
	execPath, err := config.SelfPath()
	if err != nil {
		return err
	}

	if err := writeSocksCredsFile(users); err != nil {
		return fmt.Errorf("write creds file: %w", err)
	}

	args := fmt.Sprintf("%s socks serve --addr %s --port 1080 --creds-file %s", execPath, listenAddr, socksCredsFile)
	if requireAuth {
		args += " --require-auth"
	}

	// Clean up old microsocks service if it exists
	_ = service.Stop("rovunet-microsocks")
	_ = service.Remove("rovunet-microsocks")

	svcUser := config.SystemUser
	if RunAsUser != "" {
		svcUser = RunAsUser
	}

	unit := &service.Unit{
		Name:        socksServiceName,
		Description: "Rovunet Panel SOCKS5 proxy",
		ExecStart:   args,
		ExecReload:  "/bin/kill -HUP $MAINPID",
		User:        svcUser,
		Group:       config.SystemGroup,
		After:       "network.target",
		Restart:     "always",
	}

	// If the installed unit file already matches the desired ExecStart and
	// User, the only thing that changed is the creds file contents. Hot-reload
	// via SIGHUP so existing client connections survive.
	if existing := service.ReadUnitFile(socksServiceName); existing != "" &&
		strings.Contains(existing, "ExecStart="+args+"\n") &&
		strings.Contains(existing, "User="+svcUser+"\n") {
		// Reload only a running service; otherwise systemctl prints
		// "is not active, cannot reload" and we restart below anyway.
		if st, _ := service.Status(socksServiceName); st == "active" {
			if err := service.Reload(socksServiceName); err == nil {
				return nil
			}
		}
		// Fall through to recreate+restart if reload fails (e.g. service not running yet).
	}

	if err := service.Create(unit); err != nil {
		return err
	}

	// New or changed unit: pick up the new ExecStart with a restart.
	_ = service.Restart(unit.Name)
	return service.Start(unit.Name)
}

// writeSocksCredsFile writes the current credential set to socksCredsFile
// atomically (temp file + rename) so a concurrent SIGHUP reader never sees
// a partial file.
func writeSocksCredsFile(users []config.UserConfig) error {
	if os.Geteuid() == 0 {
		if err := system.EnsureDir(filepath.Dir(socksCredsFile), "root"); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(socksCredsFile), 0750); err != nil {
		return err
	}

	var buf strings.Builder
	for _, u := range users {
		if u.Username != "" && u.Password != "" {
			fmt.Fprintf(&buf, "%s:%s\n", u.Username, u.Password)
		}
	}

	uid, gid := -1, -1
	if os.Geteuid() == 0 {
		account, err := user.Lookup(config.SystemUser)
		if err != nil {
			return fmt.Errorf("resolve SOCKS service user: %w", err)
		}
		group, err := user.LookupGroup(config.SystemGroup)
		if err != nil {
			return fmt.Errorf("resolve SOCKS service group: %w", err)
		}
		uid, err = strconv.Atoi(account.Uid)
		if err != nil {
			return err
		}
		gid, err = strconv.Atoi(group.Gid)
		if err != nil {
			return err
		}
	} else {
		log.Printf("warning: not running as root, keeping caller ownership for %s", socksCredsFile)
	}
	return writeSocksCredentials(socksCredsFile, []byte(buf.String()), uid, gid)
}

// A unique private staging file avoids following a stale fixed .tmp symlink.
// Ownership and permissions are durable before publication or SIGHUP.
func writeSocksCredentials(path string, data []byte, uid, gid int) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("SOCKS credentials destination must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".rovunet-socks-*")
	if err != nil {
		return err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	if err = f.Chmod(0640); err != nil {
		return err
	}
	if uid >= 0 || gid >= 0 {
		if err = f.Chown(uid, gid); err != nil {
			return fmt.Errorf("set SOCKS credential ownership: %w", err)
		}
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	return safefile.SyncDir(filepath.Dir(path))
}
