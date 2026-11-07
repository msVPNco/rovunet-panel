package handlers

import (
	"fmt"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
)

// serverAddressAsked records that this run already asked for the server's
// public address, so a skipped question isn't asked again.
var serverAddressAsked bool

// ensureServerAddress makes sure configs for tunnels that clients reach at
// the server's address (StunTLS, direct SSH/SOCKS5) get a public one. When
// none can be found it asks once, saving the answer, and otherwise warns.
func ensureServerAddress(cfg *config.Config, out actions.OutputWriter, tunnels []config.TunnelConfig) {
	needed := false
	for i := range tunnels {
		needed = needed || tunnels[i].ConnectsByAddress()
	}
	if !needed {
		return
	}
	addr, public := clientcfg.ServerAddress(cfg)
	if public {
		return
	}
	if addr == "" {
		addr = "unknown"
	}

	if !serverAddressAsked {
		serverAddressAsked = true
		out.Print("")
		out.Warning(fmt.Sprintf("This server's IP (%s) is private, so clients can't reach it. Cloud servers such as Oracle and AWS only see their private IP.", addr))
		answer, err := prompt.String("This server's public IP or hostname for client configs (blank to skip)", "")
		if answer = strings.TrimSpace(answer); err == nil && answer != "" {
			if err := setPublicAddress(cfg, answer); err != nil {
				out.Warning(err.Error())
			} else {
				out.Success("Client configs will use " + answer)
				return
			}
		}
	}
	out.Warning(fmt.Sprintf("StunTLS, SSH and SOCKS5 configs use the private IP %s. Set the public one with: sudo rovunet config address <ip>", addr))
}

// setPublicAddress validates and saves the address client configs use and any
// extra ones. The text is one address, or several separated by commas: the
// first is the main address ("auto" detects it), the rest are extra addresses
// of the same server. "auto" alone clears everything.
func setPublicAddress(cfg *config.Config, text string) error {
	var addrs []string
	for _, a := range strings.Split(text, ",") {
		if a = strings.TrimSpace(a); a != "" {
			addrs = append(addrs, a)
		}
	}
	if len(addrs) == 0 {
		return fmt.Errorf("no address given")
	}
	main, extra := addrs[0], addrs[1:]
	if main == "auto" {
		main = ""
	} else if err := config.ValidatePublicAddress(main); err != nil {
		return err
	}
	seen := map[string]bool{main: true}
	var uniq []string
	for _, a := range extra {
		if err := config.ValidatePublicAddress(a); err != nil {
			return err
		}
		if !seen[a] {
			seen[a] = true
			uniq = append(uniq, a)
		}
	}
	oldMain, oldExtra := cfg.PublicAddress, cfg.ExtraAddresses
	cfg.PublicAddress, cfg.ExtraAddresses = main, uniq
	if err := cfg.Save(); err != nil {
		cfg.PublicAddress, cfg.ExtraAddresses = oldMain, oldExtra
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// addressList shows the main address followed by the extra ones.
func addressList(cfg *config.Config) string {
	list := cfg.PublicAddress
	if list == "" {
		list = "auto"
	}
	if len(cfg.ExtraAddresses) > 0 {
		list += "," + strings.Join(cfg.ExtraAddresses, ",")
	}
	return list
}

func handleConfigAddress(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	out := ctx.Output
	addr := strings.TrimSpace(ctx.GetArg("address"))

	if addr == "" {
		current, public := clientcfg.ServerAddress(cfg)
		switch {
		case cfg.PublicAddress != "":
			out.Info("Client configs use " + current + " (set by you)")
		case public:
			out.Info("Client configs use " + current + " (detected)")
		default:
			out.Warning(fmt.Sprintf("Detected %q, which is private: clients can't reach it", current))
		}
		if len(cfg.ExtraAddresses) > 0 {
			out.Info("Extra addresses: " + strings.Join(cfg.ExtraAddresses, ", "))
		}
		var err error
		addr, err = prompt.String(`Public IP or hostname ("auto" to detect; add more of this server's IPs after commas)`, addressList(cfg))
		if err != nil {
			return err
		}
		if addr = strings.TrimSpace(addr); addr == "" || addr == addressList(cfg) {
			return nil
		}
	}

	if err := setPublicAddress(cfg, addr); err != nil {
		return actions.NewError(actions.ConfigAddress, err.Error(), nil)
	}
	now, public := clientcfg.ServerAddress(cfg)
	if public {
		out.Success("Client configs now use " + now)
	} else {
		out.Warning(fmt.Sprintf("Client configs now use the detected private IP %s, which clients can't reach", now))
	}
	if len(cfg.ExtraAddresses) > 0 {
		out.Success("Subscription links also carry each address-based config for: " + strings.Join(cfg.ExtraAddresses, ", "))
		out.Info("Every extra address must be an IP or hostname of THIS server, with the same ports open.")
		if err := WriteSubscriptions(cfg, out); err != nil {
			out.Warning("Failed to refresh subscription links: " + err.Error())
		}
	}
	out.Info("Configs handed out before keep the old address; show the new ones with: sudo rovunet users configs")
	return nil
}
