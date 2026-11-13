package dnsrouter

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

const serviceName = "rovunet-dnsrouter"

// CreateRouterService creates and enables the systemd service for the DNS router.
func CreateRouterService() error {
	return applyRouterService(false)
}

// RegenerateRouterService keeps an administrator's stopped/disabled choice.
func RegenerateRouterService() error { return applyRouterService(true) }

func applyRouterService(preserve bool) error {
	execPath, err := config.SelfPath()
	if err != nil {
		return err
	}

	unit := &service.Unit{
		Name:            serviceName,
		Description:     "Rovunet Panel DNS Router",
		ExecStart:       execPath + " dnsrouter serve",
		User:            "root", // needs to bind port 53
		Group:           config.SystemGroup,
		After:           "network.target systemd-resolved.service",
		Restart:         "always",
		NoNewPrivileges: true,
	}

	return service.Apply(unit, preserve)
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
