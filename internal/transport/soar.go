package transport

import (
	"fmt"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

func createSoarService(tunnel *config.TunnelConfig, cfg *config.Config) error {
	execStart, err := buildSoarExecStart(tunnel, cfg)
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

	if err := service.Create(unit); err != nil {
		return err
	}
	// Restart if already running: systemctl start on an already-active unit
	// is a no-op, so an edit to this tunnel's settings would otherwise sit
	// in the rewritten unit file without ever reaching the running process.
	if _, err := service.Status(unit.Name); err == nil {
		return service.Restart(unit.Name)
	}
	return service.Start(unit.Name)
}

// buildSoarExecStart builds the ExecStart command for the built-in soar server.
func buildSoarExecStart(tunnel *config.TunnelConfig, cfg *config.Config) (string, error) {
	if tunnel.Soar == nil {
		return "", fmt.Errorf("soar config is nil")
	}

	// rovunet binary itself serves soar — no external binary needed
	execPath, err := config.SelfPath()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	// Loopback only: queries reach soar through the DNS router on port 53.
	execStart := fmt.Sprintf("%s soar serve --listen 127.0.0.1:%d --zone %s --key-file %s",
		execPath, tunnel.Port, tunnel.Domain, tunnel.Soar.PrivateKey)
	if tunnel.Backend == config.BackendSSH {
		// Streams may reach the SSH backend and nothing else.
		backend := cfg.GetBackend(config.BackendSSH)
		if backend == nil {
			return "", fmt.Errorf("backend %q not found", config.BackendSSH)
		}
		return execStart + " --backend " + backend.Address, nil
	}
	return execStart + " --allow-ports " + tunnel.Soar.ResolvedAllowPorts(), nil
}
