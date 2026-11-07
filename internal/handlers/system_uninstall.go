package handlers

import (
	"os"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/system"
	"github.com/msVPNco/rovunet-panel/internal/warp"
)

func handleSystemUninstall(ctx *actions.Context) error {
	out := ctx.Output

	ok, err := prompt.Confirm("This will remove ALL tunnels, services, configs, and the rovunet user. Continue?")
	if err != nil {
		return err
	}
	if !ok {
		out.Info("Cancelled")
		return nil
	}

	// Stop and remove ALL rovunet services (config + any orphaned ones)
	for _, svcName := range service.ListRovunetServices() {
		out.Info("Stopping " + svcName + "...")
		_ = service.Stop(svcName)
		_ = service.Remove(svcName)
	}

	// Also clean up legacy microsocks service
	_ = service.Stop("rovunet-microsocks")
	_ = service.Remove("rovunet-microsocks")

	// Clean up dnstm if present
	_, _ = offerDnstmCleanup(out, actions.SystemUninstall)

	// Stop WARP and remove its dedicated users
	out.Info("Removing WARP...")
	warp.Uninstall()
	warp.RemoveUsers()

	// Remove config directory
	out.Info("Removing /etc/rovunet/...")
	if err := os.RemoveAll(config.DefaultConfigDir); err != nil {
		out.Warning("Failed to remove config dir: " + err.Error())
	}

	// Remove system user
	out.Info("Removing system user...")
	if err := system.RemoveUser(); err != nil {
		out.Warning("Failed to remove user: " + err.Error())
	}

	// Remove binaries
	out.Info("Removing binaries...")
	execPath, _ := os.Executable()
	for _, bin := range []string{
		"dnstt-server", "slipstream-server", "vaydns-server", "caddy-naive", "microsocks", "xray",
	} {
		os.Remove(config.DefaultBinDir + "/" + bin)
	}

	// Remove rovunet binary last
	if execPath != "" {
		os.Remove(execPath)
	}

	out.Success("Uninstall complete")
	return nil
}
