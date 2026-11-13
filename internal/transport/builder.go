package transport

import (
	"fmt"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

// CreateService creates and starts a systemd service for a tunnel.
func CreateService(tunnel *config.TunnelConfig, cfg *config.Config) error {
	return createService(tunnel, cfg, false)
}

// RegenerateService rewrites an owned unit and restarts it only if it was
// active. Its enable/disable state is preserved; an admin-stopped tunnel is
// never briefly started during an ordinary panel upgrade.
func RegenerateService(tunnel *config.TunnelConfig, cfg *config.Config) error {
	return createService(tunnel, cfg, true)
}

func createService(tunnel *config.TunnelConfig, cfg *config.Config, preserve bool) error {
	switch tunnel.Transport {
	case config.TransportDNSTT:
		return createDNSTTService(tunnel, cfg, preserve)
	case config.TransportSlipstream:
		return createSlipstreamService(tunnel, cfg, preserve)
	case config.TransportVayDNS:
		return createVayDNSService(tunnel, cfg, preserve)
	case config.TransportSoar:
		return createSoarService(tunnel, cfg, preserve)
	case config.TransportNaive:
		return createNaiveService(tunnel, cfg, preserve)
	case config.TransportStunTLS:
		return createStunTLSService(tunnel, cfg, preserve)
	case config.TransportVLESS:
		return createVLESSService(tunnel, cfg, preserve)
	case config.TransportSSH, config.TransportSOCKS:
		// Direct transports use existing system services (sshd, microsocks)
		return nil
	case config.TransportExternal:
		// External tunnels use a user-managed service; nothing to create
		return nil
	default:
		return fmt.Errorf("unknown transport: %s", tunnel.Transport)
	}
}

func createDNSTTService(tunnel *config.TunnelConfig, cfg *config.Config, preserve bool) error {
	execStart, err := buildDNSTTExecStart(tunnel, cfg)
	if err != nil {
		return err
	}

	unit := &service.Unit{
		Name:        service.TunnelServiceName(tunnel.Tag),
		Description: fmt.Sprintf("Rovunet Panel tunnel: %s (%s/%s)", tunnel.Tag, tunnel.Transport, tunnel.Backend),
		ExecStart:   execStart,
		User:        "root",
		Group:       config.SystemGroup,
		After:       "network.target",
		Restart:     "always",
	}

	return service.Apply(unit, preserve)
}

func createSlipstreamService(tunnel *config.TunnelConfig, cfg *config.Config, preserve bool) error {
	execStart, err := buildSlipstreamExecStart(tunnel, cfg)
	if err != nil {
		return err
	}

	unit := &service.Unit{
		Name:        service.TunnelServiceName(tunnel.Tag),
		Description: fmt.Sprintf("Rovunet Panel tunnel: %s (%s/%s)", tunnel.Tag, tunnel.Transport, tunnel.Backend),
		ExecStart:   execStart,
		User:        "root",
		Group:       config.SystemGroup,
		After:       "network.target",
		Restart:     "always",
	}

	return service.Apply(unit, preserve)
}

// RemoveService stops and removes a tunnel's systemd service.
func RemoveService(tag string) error {
	name := service.TunnelServiceName(tag)
	_ = service.Stop(name)
	return service.Remove(name)
}
