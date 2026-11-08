package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/system"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"github.com/msVPNco/rovunet-panel/internal/version"
	"github.com/msVPNco/rovunet-panel/internal/warp"
)

func handleSystemUpdate(ctx *actions.Context) error {
	out := ctx.Output
	out.Info("Current version: " + version.String())

	base := binary.DownloadBase()
	downloadURL := fmt.Sprintf("%s/rovunet-%s-%s", base, runtime.GOOS, runtime.GOARCH)
	out.Info("Downloading from: " + downloadURL)

	execPath, err := os.Executable()
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to find current binary", err)
	}

	// Download into execPath's own directory (not the system temp dir) so
	// the install below is a same-filesystem, atomic rename rather than a
	// cross-filesystem copy that can leave no binary behind if it fails
	// partway. Download also verifies the result against SHA256SUMS.
	tmpPath, err := binary.Download(downloadURL, filepath.Dir(execPath))
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to download or verify update", err)
	}
	defer os.Remove(tmpPath) // no-op once the rename below has moved it

	if runtime.GOOS == "linux" {
		backupPath := execPath + ".bak"
		if err := copyFile(execPath, backupPath); err != nil {
			return actions.NewError(actions.SystemUpdate, "failed to back up current binary before update", err)
		}
		if err := os.Chmod(backupPath, 0755); err != nil {
			out.Warning("Failed to set backup binary permissions: " + err.Error())
		}

		if err := os.Rename(tmpPath, execPath); err != nil {
			// tmpPath is in execPath's own directory (see binary.Download
			// above), so this is expected to be a same-filesystem rename
			// that always succeeds. If it still fails for some other
			// reason (e.g. a permission or SELinux denial), fall back to
			// remove-then-copy, but restore from the backup made above if
			// that also fails — so a failed update can never leave the
			// panel with no binary at all.
			os.Remove(execPath)
			if cpErr := exec.Command("cp", tmpPath, execPath).Run(); cpErr != nil {
				if restoreErr := os.Rename(backupPath, execPath); restoreErr != nil {
					return actions.NewError(actions.SystemUpdate, fmt.Sprintf(
						"install failed (%v), fallback copy failed (%v), AND restoring the backup failed (%v) — %s may now be missing; the previous binary is still at %s",
						err, cpErr, restoreErr, execPath, backupPath), err)
				}
				return actions.NewError(actions.SystemUpdate, fmt.Sprintf("failed to install update (%v); restored the previous binary", err), err)
			}
		}
		os.Chmod(execPath, 0755)
		os.Remove(backupPath)
	}

	out.Success("rovunet updated")

	// Hand off to the newly installed binary for transport updates and
	// service regeneration so the new code's URLs and logic are used.
	out.Print("")
	postCmd := exec.Command(execPath, "post-update")
	postCmd.Stdout = os.Stdout
	postCmd.Stderr = os.Stderr
	postCmd.Stdin = os.Stdin
	if err := postCmd.Run(); err != nil {
		out.Warning(fmt.Sprintf("Post-update handoff failed: %v", err))
		out.Info("Falling back to current process...")
		return runPostUpdate(ctx)
	}
	return nil
}

// HandlePostUpdate runs the post-update steps (transport binary updates,
// service regeneration). Exported for use by the hidden post-update command.
func HandlePostUpdate(ctx *actions.Context) error {
	return runPostUpdate(ctx)
}

func runPostUpdate(ctx *actions.Context) error {
	out := ctx.Output

	// Update transport binaries
	out.Info("Updating transport binaries...")

	transportBins := []string{"dnstt-server", "slipstream-server", "vaydns-server", "caddy-naive"}
	for _, name := range transportBins {
		binPath := filepath.Join(config.DefaultBinDir, name)
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			continue // not installed, skip
		}

		out.Info(fmt.Sprintf("  Updating %s...", name))

		// Backup old binary before replacing so we can restore on failure
		backupPath := binPath + ".bak"
		if err := os.Rename(binPath, backupPath); err != nil {
			out.Warning(fmt.Sprintf("  Failed to backup %s: %v", name, err))
			continue
		}
		if err := binary.EnsureInstalled(name); err != nil {
			// Restore from backup
			if restoreErr := os.Rename(backupPath, binPath); restoreErr != nil {
				out.Warning(fmt.Sprintf("  Failed to restore %s backup: %v", name, restoreErr))
			}
			out.Warning(fmt.Sprintf("  Failed to update %s: %v (kept old version)", name, err))
			continue
		}
		os.Remove(backupPath)
		out.Success(fmt.Sprintf("  %s updated", name))
	}

	// Migrate from microsocks to built-in SOCKS5 proxy
	microsocksPath := filepath.Join(config.DefaultBinDir, "microsocks")
	if _, err := os.Stat(microsocksPath); err == nil {
		out.Print("")
		out.Info("Migrating from microsocks to built-in SOCKS5 proxy...")
		cfg := ctx.Config.(*config.Config)

		// Determine listen mode from existing config
		directSOCKS := false
		for _, t := range cfg.Tunnels {
			if t.Transport == config.TransportSOCKS {
				directSOCKS = true
			}
		}

		if cfg.Warp.Enabled {
			proxy.RunAsUser = warp.SocksUser
		}
		var setupErr error
		if directSOCKS {
			setupErr = proxy.SetupSOCKSExternalWithUsers(cfg.Users)
		} else if len(cfg.Users) > 0 {
			setupErr = proxy.SetupSOCKSWithUsers(cfg.Users)
		} else {
			setupErr = proxy.SetupSOCKS()
		}
		if setupErr != nil {
			out.Warning("Failed to migrate SOCKS5 proxy: " + setupErr.Error())
		} else {
			os.Remove(microsocksPath)
			out.Success("Migrated to built-in SOCKS5 proxy")
		}
	}

	// Re-apply cap_net_bind_service to caddy-naive if WARP is enabled,
	// since the capability is lost when the binary is replaced.
	{
		cfg := ctx.Config.(*config.Config)
		if cfg.Warp.Enabled {
			naivePath := filepath.Join(config.DefaultBinDir, "caddy-naive")
			if _, err := os.Stat(naivePath); err == nil {
				if err := exec.Command("setcap", "cap_net_bind_service=+ep", naivePath).Run(); err != nil {
					out.Warning("Failed to re-set caddy-naive capability: " + err.Error())
				}
			}
		}
	}

	// Regenerate and restart all tunnel services
	// This ensures systemd unit files match the current binary interface.
	out.Print("")
	out.Info("Regenerating services...")
	cfg := ctx.Config.(*config.Config)

	// Re-open every tunnel's ports: this also frees port 53 from the
	// systemd-resolved stub (OS updates can re-enable it), opens 53/tcp, which
	// the DNS router serves since TCP forwarding was added, and moves rules an
	// older version appended behind a REJECT rule to where they take effect.
	openTunnelPorts(cfg, out)

	// Bring the SSH tunnel users' sshd settings up to date (e.g. password
	// logins on images that turn them off globally)
	if len(cfg.Users) > 0 {
		if err := system.EnsureSSHConfig(); err != nil {
			out.Warning("Failed to update SSH settings: " + err.Error())
		}
	}

	// Regenerate and restart tunnel services first
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if t.IsDirectTransport() {
			continue
		}
		svcName := service.TunnelServiceName(t.Tag)
		wasActive := false
		if status, _ := service.Status(svcName); status == "active" {
			wasActive = true
			_ = service.Stop(svcName)
		}
		if err := transport.CreateService(t, cfg); err != nil {
			out.Warning(fmt.Sprintf("Failed to regenerate %s: %v", svcName, err))
			continue
		}
		if wasActive {
			out.Success(fmt.Sprintf("  %s regenerated and restarted", svcName))
		} else {
			out.Success(fmt.Sprintf("  %s regenerated", svcName))
		}
	}

	// Recreate SOCKS5 proxy service to pick up new ExecStart format (e.g. --creds)
	if service.Exists("rovunet-socks5") {
		if cfg.Warp.Enabled {
			proxy.RunAsUser = warp.SocksUser
		}
		if setupErr := setupSOCKS(cfg); setupErr != nil {
			out.Warning("Failed to regenerate SOCKS5 proxy: " + setupErr.Error())
		} else {
			out.Success("  rovunet-socks5 regenerated and restarted")
		}
	}

	// Apply the rule that a traffic-limited user may use only counted paths to
	// users limited by an older version: no SSH account, no Hysteria 2 or
	// WireGuard credential.
	syncSSHAccess(cfg, out)
	syncCores(cfg, out)

	// Restart DNS router last so tunnels are ready
	if service.Exists("rovunet-dnsrouter") {
		if err := service.Restart("rovunet-dnsrouter"); err != nil {
			out.Warning(fmt.Sprintf("Failed to restart rovunet-dnsrouter: %v", err))
		} else {
			out.Success("  rovunet-dnsrouter restarted")
		}
	}

	// Subscription files: one per user, served by the NaïveProxy Caddy.
	if err := WriteSubscriptions(cfg, out); err != nil {
		out.Warning("Failed to write subscription files: " + err.Error())
	}

	out.Print("")
	out.Success("Update complete!")
	return nil
}
