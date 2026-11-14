package handlers

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/network"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/system"
	"github.com/msVPNco/rovunet-panel/internal/warp"
)

var uninstallDataPaths = []string{"/etc/rovunet", "/var/lib/rovunet", "/var/lib/rovunet-quota"}

func archiveUninstallData(root string, paths []string) (string, error) {
	dir, err := os.MkdirTemp(root, "rovunet-uninstall-recovery-")
	if err != nil {
		return "", err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return "", err
	}
	archivePath := filepath.Join(dir, "private-data.tar.gz")
	f, err := os.OpenFile(archivePath+".partial", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, root := range paths {
		if _, e := os.Lstat(root); os.IsNotExist(e) {
			continue
		}
		err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Stopped runtimes can leave Unix-domain socket entries behind.
			// They contain no restorable state and archive/tar rejects them;
			// durable configuration, SQLite/WAL, keys and symlinks remain covered.
			if info.Mode()&os.ModeSocket != 0 {
				return nil
			}
			link := ""
			if info.Mode()&os.ModeSymlink != 0 {
				link, err = os.Readlink(path)
				if err != nil {
					return err
				}
			}
			header, err := tar.FileInfoHeader(info, link)
			if err != nil {
				return err
			}
			header.Name = strings.TrimPrefix(filepath.ToSlash(path), "/")
			if err = tw.WriteHeader(header); err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				src, err := os.Open(path)
				if err != nil {
					return err
				}
				_, copyErr := io.Copy(tw, src)
				return errors.Join(copyErr, src.Close())
			}
			return nil
		})
		if err != nil {
			_ = tw.Close()
			_ = gz.Close()
			_ = f.Close()
			return "", err
		}
	}
	if err = errors.Join(tw.Close(), gz.Close(), f.Sync(), f.Close()); err != nil {
		return "", err
	}
	if err := os.Rename(archivePath+".partial", archivePath); err != nil {
		return "", err
	}
	if err = errors.Join(safefile.SyncDir(dir), safefile.SyncDir(root)); err != nil {
		return "", err
	}
	return dir, nil
}

func uninstallBinaryNames(entries []os.DirEntry) []string {
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if strings.HasPrefix(name, ".rovunet-install.") {
				names = append(names, name)
			}
			continue
		}
		for _, base := range []string{"rovunet", "xray", "dnstt-server", "slipstream-server", "slipstream-client", "vaydns-server", "caddy-naive", "microsocks"} {
			if name == base || strings.HasPrefix(name, base+".") || (base == "rovunet" && strings.HasPrefix(name, "rovunet-")) || strings.HasPrefix(name, ".rovunet-") {
				names = append(names, name)
				break
			}
		}
	}
	return names
}

func verifiedUninstallFiles(dir string, entries []os.DirEntry, owned map[string]string) (remove, keep []string) {
	for _, name := range uninstallBinaryNames(entries) {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || owned[path] == "" {
			keep = append(keep, name)
			continue
		}
		hash, err := binary.FileSHA256(path)
		if err != nil || hash != owned[path] {
			keep = append(keep, name)
			continue
		}
		remove = append(remove, name)
	}
	return
}

func addVerifiedBootstrapOwnership(owned map[string]string) error {
	path := filepath.Join(config.DefaultBinDir, ".rovunet-install-state.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("unsafe bootstrap receipt retained")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Uid != 0 {
		return errors.New("bootstrap receipt is not root-owned")
	}
	var receipt struct {
		Schema int    `json:"schema"`
		Phase  string `json:"phase"`
		SHA256 string `json:"sha256"`
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(b) > 65536 || json.Unmarshal(b, &receipt) != nil || receipt.Schema != 1 || receipt.Phase != "verified-before-replace" {
		return errors.New("unrecognized bootstrap receipt retained")
	}
	target := filepath.Join(config.DefaultBinDir, "rovunet")
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	hash, err := binary.FileSHA256(target)
	if err != nil || target != exe || hash != receipt.SHA256 {
		return errors.New("bootstrap executable binding changed; retained")
	}
	owned[target] = hash
	owned[path], err = binary.FileSHA256(path)
	return err
}

func uninstallFileInUse(path string) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return true
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err == nil && strings.TrimSuffix(exe, " (deleted)") == path {
			return true
		}
	}
	return false
}

func handleSystemUninstall(ctx *actions.Context) error {
	out := ctx.Output
	ok, err := prompt.Confirm("Remove all Rovunet tunnels, services, timers, active config, accounting data and owned tunnel users? A private recovery archive will be kept under /root. Continue?")
	if err != nil || !ok {
		return err
	}
	if err := service.RemovePanelUnits(); err != nil {
		return fmt.Errorf("uninstall pending; data and executable retained: %w", err)
	}
	ownership, err := binary.ReadInstalledOwnership()
	if err != nil {
		return fmt.Errorf("uninstall pending; untrusted ownership receipt retained: %w", err)
	}
	if err := addVerifiedBootstrapOwnership(ownership.Files); err != nil {
		return err
	}
	// Stop every panel writer before copying SQLite/WAL and private keys.
	archivePaths := append(append([]string{}, uninstallDataPaths...), "/etc/ssh/sshd_config", "/etc/systemd/resolved.conf.d/rovunet-no-stub.conf", "/etc/resolv.conf")
	archive, err := archiveUninstallData("/root", archivePaths)
	if err != nil {
		return fmt.Errorf("recovery archive failed; active data retained: %w", err)
	}
	out.Info("Private recovery archive (contains secrets; keep private): " + archive)
	if err := system.RemoveOwnedSSHConfig(); err != nil {
		return fmt.Errorf("uninstall pending; config and executable retained: %w", err)
	}
	if err := network.RemoveResolvedStubDropin(); err != nil {
		return fmt.Errorf("uninstall pending; custom DNS and panel data retained: %w", err)
	}
	if err := system.RemoveOwnedTunnelAccounts(); err != nil {
		return fmt.Errorf("uninstall pending; config and executable retained: %w", err)
	}
	if _, err := offerDnstmCleanup(out, actions.SystemUninstall); err != nil {
		return err
	}
	warp.Uninstall()
	warp.RemoveUsers()
	if err := system.RemovePanelServiceAccounts(); err != nil {
		return err
	}
	for _, path := range uninstallDataPaths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	for _, path := range []string{"/run/rovunet-subscription-v2", "/run/rovunet", "/etc/rovunet/config.json.accounting"} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(config.DefaultBinDir)
	if err != nil {
		return err
	}
	remove, keep := verifiedUninstallFiles(config.DefaultBinDir, entries, ownership.Files)
	// Keep the running executable until all other requested removals succeed.
	for _, name := range remove {
		if name == "rovunet" {
			continue
		}
		path := filepath.Join(config.DefaultBinDir, name)
		if uninstallFileInUse(path) {
			keep = append(keep, name)
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	for _, name := range remove {
		if name == "rovunet" {
			path := filepath.Join(config.DefaultBinDir, name)
			if uninstallFileInUse(path) {
				keep = append(keep, name)
			} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, name := range keep {
		out.Warning("Preserved unowned, changed or still-used shared-bin object: " + filepath.Join(config.DefaultBinDir, name))
	}
	out.Success("Owned panel data and units removed. Unknown/shared files and the private recovery archive were retained: " + archive)
	out.Info("Shared firewall rules, package dependencies and administrator DNS files were retained for review.")
	return nil
}
