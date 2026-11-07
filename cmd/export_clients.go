package cmd

import (
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/spf13/cobra"
)

func init() {
	var user, format string
	var annotated bool
	command := &cobra.Command{
		Use: "export-clients", Short: "Export client links (contains credentials; keep output private)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			u := cfg.GetUser(user)
			if u == nil {
				return fmt.Errorf("unknown user")
			}
			if format != "auto" && format != "standard" && format != "veyra" && format != "rovunet" && format != "legacy" {
				return fmt.Errorf("invalid format")
			}
			count := 0
			for i := range cfg.Tunnels {
				t := &cfg.Tunnels[i]
				if !t.Enabled {
					continue
				}
				modes := []string{"dnstt"}
				if t.Transport == config.TransportDNSTT {
					modes = append(modes, "noizdns")
				}
				for _, mode := range modes {
					link, err := clientcfg.GeneratePortableURI(t, cfg.GetBackend(t.Backend), cfg, clientcfg.URIOptions{Username: u.Username, Password: u.Password, ClientMode: mode}, format)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", t.Tag, err)
						continue
					}
					if annotated {
						fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", t.Tag, mode, link)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), link)
					}
					count++
				}
			}
			// Xray cores: wrapped into the user's collection for the app, or
			// the standard link itself. A WireGuard file spans several lines,
			// so it has no standard one-line form here.
			if format == "auto" || format == "rovunet" || format == "standard" {
				profiles, err := coreinstall.Profiles(*u)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "xray cores: %v\n", err)
				}
				for _, p := range profiles {
					link := p.Link
					if format == "standard" {
						if p.Protocol == "wireguard" {
							continue
						}
					} else if link, err = clientcfg.GenerateCoreURI(p.Core, p.Link, cfg, clientcfg.URIOptions{Username: u.Username}); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", p.Core, err)
						continue
					}
					if annotated {
						fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", p.Core, p.Protocol, link)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), link)
					}
					count++
				}
			}
			if count == 0 {
				return fmt.Errorf("no compatible enabled tunnels exported")
			}
			return nil
		},
	}
	command.Flags().StringVar(&user, "user", "", "Existing managed username")
	command.Flags().StringVar(&format, "format", "auto", "auto, standard, rovunet, veyra (compatibility), or legacy")
	command.Flags().BoolVar(&annotated, "annotated", false, "Prefix each line with its tunnel tag and client mode, e.g. \"tag (dnstt): rovunet://...\". Leave off for scripts, which parse one bare link per line")
	_ = command.MarkFlagRequired("user")
	rootCmd.AddCommand(command)
}
