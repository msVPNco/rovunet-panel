package handlers

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/dnsrouter"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/system"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"github.com/msVPNco/rovunet-panel/internal/version"
	"github.com/msVPNco/rovunet-panel/internal/warp"
)

func handleSystemUpdate(ctx *actions.Context) error {
	out := ctx.Output
	cfg := ctx.Config.(*config.Config)
	if cfg.RecoverySealed {
		return config.ErrCoordinatedRecovery
	}
	out.Info("Current version: " + version.String())
	if strings.Contains(version.Version, "managed-preview") {
		return actions.NewError(actions.SystemUpdate, "this private preview requires a reviewed preview package or its rollback script; the public updater does not yet support its managed cores", nil)
	}

	if runtime.GOOS != "linux" {
		return actions.NewError(actions.SystemUpdate, "only Linux servers are supported", nil)
	}
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(filepath.Dir(execPath), ".rovunet-update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if tx, resume, err := binary.ResumeUpdate(execPath); err != nil {
		return err
	} else if resume {
		out.Info("Resuming the installed panel's pending setup; recovery files are retained.")
		if err := runStrictPostUpdate(execPath, tx); err != nil {
			return err
		}
		out.Success("Update complete!")
		return nil
	}
	newVersion, downloadURL, err := binary.CheckUpdate()
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to resolve release", err)
	}
	if newVersion == "" {
		out.Info("The installed panel already matches the selected release.")
		if cfg.SubscriptionV2 || requiresCoherent(cfg) {
			return HandlePostUpdate(ctx)
		}
		if err := completeAfterUpdate(ctx, cfg); err != nil {
			return err
		}
		if err := coreinstall.ConfigureDefaultHysteriaObfuscation(cfg.Users); err != nil {
			return err
		}
		if err := WriteSubscriptions(cfg, out); err != nil {
			return err
		}
		return finishInstallationAccounting(cfg, nil)
	}
	out.Info("Staging authenticated release " + newVersion + "...")
	tmpPath, err := binary.Download(downloadURL, filepath.Dir(execPath))
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "failed to authenticate update; existing panel was retained", err)
	}
	defer os.Remove(tmpPath)
	recovery, err := prepareCandidateRecovery(cfg, tmpPath)
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "could not prepare validated recovery snapshot; existing panel was retained", err)
	}
	tx, err := binary.BeginUpdate(execPath, tmpPath, recovery)
	if err != nil {
		return actions.NewError(actions.SystemUpdate, "installation did not complete; retain the update recovery record", err)
	}
	if err := runStrictPostUpdate(execPath, tx); err != nil {
		return err
	}
	out.Success("Update complete!")
	out.Info("Private recovery snapshot: " + recovery)
	return nil
}

// HandlePostUpdate runs the post-update steps (transport binary updates,
// service regeneration). Exported for use by the hidden post-update command.
func HandlePostUpdate(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	if cfg.SubscriptionV2 || cfg.V2Authority != nil {
		if e := ValidateV2Authority(cfg); e != nil {
			return e
		}
	}
	if cfg.SubscriptionV2 || requiresCoherent(cfg) {
		if e := coreinstall.RefreshInstalledEngines(); e != nil {
			return e
		}
		if e := RecoverBeforeServing(cfg); e != nil {
			return e
		}
		if e := refreshCoherentTransports(ctx); e != nil {
			return e
		}
		if e := ApplyV2Deployment(cfg); e != nil {
			return e
		}
		if completeBaseTunnels(cfg) {
			if e := EnsureCompleteSetup(cfg, ctx.Output); e != nil {
				return e
			}
		}
		if e := finishInstallationAccounting(cfg, nil); e != nil {
			return e
		}
		return WriteSubscriptions(cfg, ctx.Output)
	}
	if err := coreinstall.RefreshInstalledEngines(); err != nil {
		return err
	}
	if err := runPostUpdate(ctx); err != nil {
		return err
	}
	return finishInstallationAccounting(cfg, nil)
}

// SQLite-backed installations still need the reviewed transport executable and
// unit migrations. Keep the legacy account synchronizer out of this path: the
// coordinated deployment below owns credentials and accounting admission.
func refreshCoherentTransports(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	var problems []error
	for _, name := range []string{"dnstt-server", "slipstream-server", "vaydns-server", "caddy-naive"} {
		path := filepath.Join(config.DefaultBinDir, name)
		if _, e := binary.RecoverInterrupted(path); e != nil {
			problems = append(problems, fmt.Errorf("%s recovery: %w", name, e))
			continue
		}
		if _, e := os.Lstat(path); os.IsNotExist(e) {
			continue
		} else if e != nil {
			problems = append(problems, e)
			continue
		}
		if e := binary.UpdateInstalled(name); e != nil {
			problems = append(problems, fmt.Errorf("%s reviewed runtime update pending: %w", name, e))
		}
	}
	if e := errors.Join(problems...); e != nil {
		return e
	}
	if cfg.Warp.Enabled {
		path := filepath.Join(config.DefaultBinDir, "caddy-naive")
		if _, e := os.Stat(path); e == nil {
			if e := exec.Command("setcap", "cap_net_bind_service=+ep", path).Run(); e != nil {
				return fmt.Errorf("restore TLS front capability: %w", e)
			}
		}
	}
	openTunnelPorts(cfg, ctx.Output)
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if !t.Enabled || t.IsDirectTransport() {
			continue
		}
		if e := transport.RegenerateService(t, cfg); e != nil {
			return e
		}
	}
	if e := refreshInstalledSOCKSWorker(); e != nil {
		return e
	}
	if service.Exists("rovunet-dnsrouter") {
		return dnsrouter.RegenerateRouterService()
	}
	return nil
}

// A credential reload cannot replace the executing panel image after an atomic
// binary update. Restart running SOCKS workers without enabling stopped units.
func refreshInstalledSOCKSWorker() error {
	const name = "rovunet-socks5"
	if !service.Exists(name) {
		return nil
	}
	state, err := service.Status(name)
	switch state {
	case "active", "reloading", "activating":
		if err := service.Restart(name); err != nil {
			return fmt.Errorf("refresh installed SOCKS worker: %w", err)
		}
		return nil
	case "inactive", "failed", "deactivating", "unknown":
		return nil
	default:
		if err != nil {
			return fmt.Errorf("cannot determine installed SOCKS worker state %q: %w", state, err)
		}
		return fmt.Errorf("unexpected installed SOCKS worker state %q", state)
	}
}

func runPostUpdate(ctx *actions.Context) error {
	out := ctx.Output
	var problems []error
	warn := func(message string) { out.Warning(message); problems = append(problems, errors.New(message)) }

	// Update transport binaries
	out.Info("Updating transport binaries...")

	transportBins := []string{"dnstt-server", "slipstream-server", "vaydns-server", "caddy-naive"}
	for _, name := range transportBins {
		binPath := filepath.Join(config.DefaultBinDir, name)
		if restored, err := binary.RecoverInterrupted(binPath); err != nil {
			warn(fmt.Sprintf("%s recovery: %v", name, err))
		} else if restored {
			out.Warning(name + " was restored from its interrupted-update backup")
		}
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			continue
		}
		if err := binary.UpdateInstalled(name); err != nil {
			warn(fmt.Sprintf("%s update failed; its existing executable or recovery binary was retained: %v", name, err))
			continue
		}
		out.Info(name + " pinned runtime installed; recovery binary retained")
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
			warn("Failed to migrate SOCKS5 proxy: " + setupErr.Error())
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
					warn("Failed to re-set caddy-naive capability: " + err.Error())
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
			warn("Failed to update SSH settings: " + err.Error())
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
		}
		if err := transport.RegenerateService(t, cfg); err != nil {
			warn(fmt.Sprintf("Failed to regenerate %s: %v", svcName, err))
			continue
		}
		if wasActive {
			out.Success(fmt.Sprintf("  %s regenerated and restarted", svcName))
		} else {
			out.Success(fmt.Sprintf("  %s regenerated", svcName))
		}
	}

	// Recreate SOCKS5 proxy service to pick up new ExecStart format (e.g. --creds)
	if service.Exists("rovunet-socks5") && activeService("rovunet-socks5") {
		if cfg.Warp.Enabled {
			proxy.RunAsUser = warp.SocksUser
		}
		if setupErr := setupSOCKS(cfg); setupErr != nil {
			warn("Failed to regenerate SOCKS5 proxy: " + setupErr.Error())
		} else if e := refreshInstalledSOCKSWorker(); e != nil {
			warn("Failed to refresh SOCKS5 executable: " + e.Error())
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
		if err := dnsrouter.RegenerateRouterService(); err != nil {
			warn(fmt.Sprintf("Failed to restart rovunet-dnsrouter: %v", err))
		} else {
			out.Info("  rovunet-dnsrouter regenerated; active and enable choices preserved")
		}
	}

	// Subscription files: one per user, served by the NaïveProxy Caddy.
	if err := WriteSubscriptions(cfg, out); err != nil {
		warn("Failed to write subscription files: " + err.Error())
	}

	out.Print("")
	if err := completeAfterUpdate(ctx, cfg); err != nil {
		// v1.21.0's updater replays its OLD migration if post-update exits
		// nonzero. Never allow it to rewrite the new runtime options. Report
		// the pending setup explicitly, and expose a retry-only command.
		out.Error("Panel updated; complete setup is pending: " + err.Error())
		out.Info("After fixing the reported prerequisite, run: rovunet complete-setup")
		problems = append(problems, err)
	}
	// Existing clients receive the new transport options at the same saved
	// URL. A failed migration keeps its private recovery and stays pending.
	if err := coreinstall.ConfigureDefaultHysteriaObfuscation(cfg.Users); err != nil {
		warn("Hy2 compatibility migration pending: " + err.Error())
	}
	if err := WriteSubscriptions(cfg, out); err != nil {
		warn("Subscription refresh pending after compatibility migration: " + err.Error())
	}
	return errors.Join(problems...)
}
