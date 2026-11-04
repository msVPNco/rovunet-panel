package handlers

import (
	"os"

	"github.com/anonvector/slipgate/internal/actions"
	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/prompt"
	"github.com/anonvector/slipgate/internal/service"
	"github.com/anonvector/slipgate/internal/system"
	"github.com/anonvector/slipgate/internal/warp"
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

	// Stop and remove ALL slipgate services (config + any orphaned ones)
	for _, svcName := range service.ListPanelServices() {
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
	// A server moved from the old SlipGate names keeps a link there
	if fi, err := os.Lstat("/etc/slipgate"); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		os.Remove("/etc/slipgate")
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
		"dnstt-server", "slipstream-server", "vaydns-server", "caddy-naive", "microsocks",
	} {
		os.Remove(config.DefaultBinDir + "/" + bin)
	}

	// Remove the panel binary last, with the old name's link if there is one
	if fi, err := os.Lstat(config.DefaultBinDir + "/slipgate"); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		os.Remove(config.DefaultBinDir + "/slipgate")
	}
	if execPath != "" {
		os.Remove(execPath)
	}

	out.Success("Uninstall complete")
	return nil
}
