package router

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

// ServiceName returns the systemd service name for a tunnel.
func ServiceName(tag string) string {
	return service.TunnelServiceName(tag)
}

// AllocatePort assigns the next available port for a DNS tunnel.
func AllocatePort(cfg *config.Config) int {
	return cfg.NextAvailablePort()
}
