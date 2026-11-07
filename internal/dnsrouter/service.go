package dnsrouter

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

const serviceName = "rovunet-dnsrouter"

// CreateRouterService creates and enables the systemd service for the DNS router.
func CreateRouterService() error {
	execPath, err := config.SelfPath()
	if err != nil {
		return err
	}

	unit := &service.Unit{
		Name:        serviceName,
		Description: "Rovunet Panel DNS Router",
		ExecStart:   execPath + " dnsrouter serve",
		User:        "root", // needs to bind port 53
		Group:       config.SystemGroup,
		After:       "network.target systemd-resolved.service",
		Restart:     "always",
	}

	if err := service.Create(unit); err != nil {
		return err
	}
	return service.Start(serviceName)
}

// StartRouterService starts the DNS router service.
func StartRouterService() error {
	return service.Start(serviceName)
}

// StopRouterService stops the DNS router service.
func StopRouterService() error {
	return service.Stop(serviceName)
}

// RestartRouterService restarts the DNS router service.
func RestartRouterService() error {
	return service.Restart(serviceName)
}
