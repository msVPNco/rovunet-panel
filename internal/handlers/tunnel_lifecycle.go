package handlers

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/certs"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

func handleTunnelStart(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")
	if tag == "" {
		return actions.NewError(actions.TunnelStart, "tunnel tag is required", nil)
	}
	tunnel := cfg.GetTunnel(tag)
	if tunnel == nil {
		return actions.NewError(actions.TunnelStart, fmt.Sprintf("tunnel %q not found", tag), nil)
	}
	svcName := service.TunnelServiceName(tag)
	if err := service.Start(svcName); err != nil {
		return actions.NewError(actions.TunnelStart, "failed to start service", err)
	}
	ctx.Output.Success(fmt.Sprintf("Tunnel %q started", tag))
	return nil
}

func handleTunnelStop(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")
	if tag == "" {
		return actions.NewError(actions.TunnelStop, "tunnel tag is required", nil)
	}
	tunnel := cfg.GetTunnel(tag)
	if tunnel == nil {
		return actions.NewError(actions.TunnelStop, fmt.Sprintf("tunnel %q not found", tag), nil)
	}
	svcName := service.TunnelServiceName(tag)
	if err := service.Stop(svcName); err != nil {
		return actions.NewError(actions.TunnelStop, "failed to stop service", err)
	}
	ctx.Output.Success(fmt.Sprintf("Tunnel %q stopped", tag))
	return nil
}

func handleTunnelStatus(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")

	if tag != "" {
		tunnel := cfg.GetTunnel(tag)
		if tunnel == nil {
			return actions.NewError(actions.TunnelStatus, fmt.Sprintf("tunnel %q not found", tag), nil)
		}
		showTunnelDetail(ctx.Output, tunnel)
		return nil
	}

	// Show all tunnels
	for i := range cfg.Tunnels {
		if err := showTunnelStatus(ctx.Output, &cfg.Tunnels[i]); err != nil {
			ctx.Output.Warning(fmt.Sprintf("Error getting status for %q: %v", cfg.Tunnels[i].Tag, err))
		}
		// DNSTT tunnels also serve noizdns clients — show a separate entry
		if cfg.Tunnels[i].Transport == config.TransportDNSTT {
			showNoizDNSStatus(ctx.Output, &cfg.Tunnels[i])
		}
	}
	// Xray cores (core-install) live outside config.json; list them too so
	// status shows everything this server serves.
	cores, err := coreinstall.List()
	if err != nil {
		ctx.Output.Warning("Failed to list Xray cores: " + err.Error())
	}
	for _, c := range cores {
		status := unitState(coreinstall.ServiceName(c.Name))
		ctx.Output.Print(fmt.Sprintf("  %-15s %-12s %-8s %-25s %s",
			c.Name, "xray-"+c.Protocol, "core", fmt.Sprintf("%d/%s", c.Port, c.Proto), status))
	}
	if len(cfg.Tunnels) == 0 && len(cores) == 0 {
		ctx.Output.Info("No tunnels configured. Run 'rovunet tunnel add' to create one.")
	}
	return nil
}

func showTunnelStatus(out actions.OutputWriter, tunnel *config.TunnelConfig) error {
	status := tunnelState(tunnel)
	out.Print(fmt.Sprintf("  %-15s %-12s %-8s %-25s %s",
		tunnel.Tag, tunnel.Transport, tunnel.Backend, tunnel.Domain, status))
	return nil
}

func showTunnelDetail(out actions.OutputWriter, tunnel *config.TunnelConfig) {
	status := tunnelState(tunnel)

	out.Print(fmt.Sprintf("  Tag       : %s", tunnel.Tag))
	out.Print(fmt.Sprintf("  Transport : %s", tunnel.Transport))
	out.Print(fmt.Sprintf("  Backend   : %s", tunnel.Backend))
	if tunnel.Domain != "" {
		out.Print(fmt.Sprintf("  Domain    : %s", tunnel.Domain))
	}
	if tunnel.Port > 0 {
		out.Print(fmt.Sprintf("  Port      : %d", tunnel.Port))
	}
	out.Print(fmt.Sprintf("  Status    : %s", status))

	switch tunnel.Transport {
	case config.TransportDNSTT:
		if tunnel.DNSTT != nil {
			out.Print(fmt.Sprintf("  MTU       : %d", tunnel.DNSTT.MTU))
			out.Print(fmt.Sprintf("  Public Key: %s", tunnel.DNSTT.PublicKey))
			if privKey := readKeyFile(tunnel.DNSTT.PrivateKey); privKey != "" {
				out.Print(fmt.Sprintf("  Priv Key  : %s", privKey))
			}
		}
	case config.TransportVayDNS:
		if tunnel.VayDNS != nil {
			out.Print(fmt.Sprintf("  MTU       : %d", tunnel.VayDNS.MTU))
			out.Print(fmt.Sprintf("  Record    : %s", tunnel.VayDNS.RecordType))
			out.Print(fmt.Sprintf("  Public Key: %s", tunnel.VayDNS.PublicKey))
			if privKey := readKeyFile(tunnel.VayDNS.PrivateKey); privKey != "" {
				out.Print(fmt.Sprintf("  Priv Key  : %s", privKey))
			}
		}
	case config.TransportSoar:
		if tunnel.Soar != nil {
			if tunnel.Backend == config.BackendDirect {
				out.Print(fmt.Sprintf("  Ports     : %s", tunnel.Soar.ResolvedAllowPorts()))
			}
			out.Print(fmt.Sprintf("  Public Key: %s", tunnel.Soar.PublicKey))
			if privKey := readKeyFile(tunnel.Soar.PrivateKey); privKey != "" {
				out.Print(fmt.Sprintf("  Priv Key  : %s", privKey))
			}
		}
	case config.TransportSlipstream:
		if tunnel.Slipstream != nil {
			out.Print(fmt.Sprintf("  Cert info : %s", certs.Describe(tunnel.Slipstream.Cert, time.Now())))
		}
	case config.TransportNaive:
		if tunnel.Naive != nil {
			out.Print(fmt.Sprintf("  Email     : %s", tunnel.Naive.Email))
			out.Print(fmt.Sprintf("  Decoy URL : %s", tunnel.Naive.DecoyURL))
		}
	case config.TransportStunTLS:
		if tunnel.StunTLS != nil {
			out.Print(fmt.Sprintf("  TLS Port  : %d", tunnel.StunTLS.Port))
			out.Print(fmt.Sprintf("  Cert      : %s", tunnel.StunTLS.Cert))
			out.Print(fmt.Sprintf("  Cert info : %s", certs.Describe(tunnel.StunTLS.Cert, time.Now())))
		}
	case config.TransportVLESS:
		if v := tunnel.VLESS; v != nil {
			security := "none (plain HTTP)"
			if v.TLS {
				security = "tls (self-signed origin cert)"
			}
			out.Print(fmt.Sprintf("  Port      : %d", v.Port))
			out.Print(fmt.Sprintf("  WS Path   : %s", v.Path))
			out.Print(fmt.Sprintf("  Security  : %s", security))
			if v.TLS {
				out.Print(fmt.Sprintf("  Cert info : %s", certs.Describe(v.Cert, time.Now())))
			}
			out.Print(fmt.Sprintf("  Behind CDN: %v", v.CDN))
			out.Print(fmt.Sprintf("  Client to : %s:%d", v.ClientAddress(tunnel.Domain), v.Port))
			out.Print(fmt.Sprintf("  Blocked   : ports %s", v.ResolvedBlockPorts()))
		}
	}
}

func readKeyFile(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// showNoizDNSStatus displays the noizdns variant for a DNSTT tunnel.
// NoizDNS shares the same server process, so it mirrors the DNSTT tunnel's status.
func showNoizDNSStatus(out actions.OutputWriter, tunnel *config.TunnelConfig) {
	status := tunnelState(tunnel)
	tag := strings.ReplaceAll(tunnel.Tag, "dnstt", "noizdns")
	out.Print(fmt.Sprintf("  %-15s %-12s %-8s %-25s %s",
		tag, "noizdns", tunnel.Backend, tunnel.Domain, status))
}

func handleTunnelLogs(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	tag := ctx.GetArg("tag")
	if tag == "" {
		return actions.NewError(actions.TunnelLogs, "tunnel tag is required", nil)
	}
	tunnel := cfg.GetTunnel(tag)
	if tunnel == nil {
		return actions.NewError(actions.TunnelLogs, fmt.Sprintf("tunnel %q not found", tag), nil)
	}
	lines := ctx.GetArg("lines")
	if lines == "" {
		lines = "50"
	}
	svcName := service.TunnelServiceName(tag)
	output, err := service.Logs(svcName, lines)
	if err != nil {
		return actions.NewError(actions.TunnelLogs, "failed to get logs", err)
	}
	ctx.Output.Print(output)
	return nil
}

// tunnelState is the state of the service that actually carries a tunnel.
// Direct SSH and SOCKS5 tunnels have no unit of their own: they are served by
// the system's sshd and by the shared SOCKS5 proxy.
func tunnelState(t *config.TunnelConfig) string {
	switch t.Transport {
	case config.TransportSSH:
		if s := unitState("ssh"); s == "active" {
			return s
		}
		return unitState("sshd")
	case config.TransportSOCKS:
		return unitState("rovunet-socks5")
	}
	return unitState(service.TunnelServiceName(t.Tag))
}

// unitState reports systemd's own word for a unit (active, activating,
// inactive, failed). systemctl exits non-zero for every state but active,
// so the error alone must not turn a failed service into "unknown".
func unitState(name string) string {
	status, _ := service.Status(name)
	if status == "" {
		return "unknown"
	}
	return status
}
