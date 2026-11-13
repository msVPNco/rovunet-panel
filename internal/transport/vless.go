package transport

import (
	"fmt"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

// createVLESSService runs the built-in VLESS over WebSocket server.
func createVLESSService(tunnel *config.TunnelConfig, cfg *config.Config, preserve bool) error {
	execStart, err := buildVLESSExecStart(tunnel, cfg)
	if err != nil {
		return err
	}

	unit := &service.Unit{
		Name:        service.TunnelServiceName(tunnel.Tag),
		Description: fmt.Sprintf("Rovunet Panel VLESS: %s", tunnel.Tag),
		ExecStart:   execStart,
		// SIGHUP reloads the users without dropping open streams
		ExecReload: "/bin/kill -HUP $MAINPID",
		User:       "root",
		Group:      config.SystemGroup,
		After:      "network-online.target",
		Restart:    "always",
	}

	return service.Apply(unit, preserve)
}

func buildVLESSExecStart(tunnel *config.TunnelConfig, cfg *config.Config) (string, error) {
	v := tunnel.VLESS
	if v == nil {
		return "", fmt.Errorf("vless config is nil")
	}
	if err := config.ValidateVLESS(v); err != nil {
		return "", err
	}

	// rovunet binary itself serves VLESS — no external binary needed
	execPath, err := config.SelfPath()
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
	var sqlBinding *config.SQLiteBinding
	for _, u := range cfg.Users {
		if u.SQLiteMeter != nil {
			if sqlBinding == nil {
				sqlBinding = u.SQLiteMeter
			} else if *sqlBinding != *u.SQLiteMeter {
				return "", fmt.Errorf("native SQL unit requires one reviewed local store")
			}
		}
	}
	if sqlBinding != nil {
		for _, u := range cfg.Users {
			if u.SQLiteMeter == nil {
				return "", fmt.Errorf("mixed native legacy/SQL listeners require separate reviewed services")
			}
		}
		if !fronted {
			return "", fmt.Errorf("native SQL unit requires a loopback Caddy front")
		}
		if containsUnitPath(sqlBinding.Directory) || containsUnitPath(sqlBinding.Store) {
			return "", fmt.Errorf("invalid SQL unit binding")
		}
		args = append(args, "--preview-quota-directory", sqlBinding.Directory, "--preview-quota-id", sqlBinding.Store)
	}
	if v.TLS && !fronted {
		args = append(args, "--cert", v.Cert, "--key", v.Key)
	}
	return strings.Join(args, " "), nil
}

func containsUnitPath(s string) bool { return s == "" || strings.ContainsAny(s, " \t\r\n%\x22") }
