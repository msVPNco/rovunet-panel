package handlers

import (
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

func markTunnelFailed(cfg *config.Config, t *config.TunnelConfig, out actions.OutputWriter) {
	t.Enabled = false
	cfg.UpdateTunnel(*t)
	if err := service.Stop(service.TunnelServiceName(t.Tag)); err != nil {
		out.Warning("Failed service could not be stopped: " + err.Error())
	}
	if err := cfg.Save(); err != nil {
		out.Warning("Could not save failed provisioning state: " + err.Error())
	}
	if err := WriteSubscriptions(cfg, out); err != nil {
		out.Warning("Could not refresh subscriptions after provisioning failure: " + err.Error())
	}
}
