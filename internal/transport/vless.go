package transport

import (
	"fmt"
	"os"
	"strings"

	"github.com/anonvector/slipgate/internal/config"
	"github.com/anonvector/slipgate/internal/service"
)

// createVLESSService runs the built-in VLESS over WebSocket server.
func createVLESSService(tunnel *config.TunnelConfig, cfg *config.Config) error {
	execStart, err := buildVLESSExecStart(tunnel, cfg)
	if err != nil {
		return err
	}

	unit := &service.Unit{
		Name:        service.TunnelServiceName(tunnel.Tag),
		Description: fmt.Sprintf("SlipGate VLESS: %s", tunnel.Tag),
		ExecStart:   execStart,
		// SIGHUP reloads the users without dropping open streams
		ExecReload: "/bin/kill -HUP $MAINPID",
		User:       "root",
		Group:      config.SystemGroup,
		After:      "network-online.target",
		Restart:    "always",
	}

	if err := service.Create(unit); err != nil {
		return err
	}
	if _, err := service.Status(unit.Name); err == nil {
		return service.Restart(unit.Name)
	}
	return service.Start(unit.Name)
}

func buildVLESSExecStart(tunnel *config.TunnelConfig, cfg *config.Config) (string, error) {
	v := tunnel.VLESS
	if v == nil {
		return "", fmt.Errorf("vless config is nil")
	}
	if err := config.ValidateVLESS(v); err != nil {
		return "", err
	}

	// slipgate binary itself serves VLESS — no external binary needed
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	usersFile := cfg.Path()
	if usersFile == "" {
		usersFile = config.DefaultConfigFile
	}

	// Behind Caddy the server is reached only through the local reverse
	// proxy; Caddy terminates TLS with its real certificate.
	listen := fmt.Sprintf("0.0.0.0:%d", v.Port)
	fronted := cfg.VLESSFront(tunnel) != nil
	if fronted {
		listen = fmt.Sprintf("127.0.0.1:%d", v.Port)
	}

	args := []string{
		execPath, "vless", "serve",
		"--listen", listen,
		"--path", v.Path,
		"--secret-file", v.SecretFile,
		"--users-file", usersFile,
		"--block-ports", v.ResolvedBlockPorts(),
	}
	if v.TLS && !fronted {
		args = append(args, "--cert", v.Cert, "--key", v.Key)
	}
	return strings.Join(args, " "), nil
}
