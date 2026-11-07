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
			// Fastest first: Xray cores are slotted in among the tunnels by
			// rank (see clientcfg.TunnelRank).
			var cores []coreinstall.Profile
			if format == "auto" || format == "rovunet" || format == "standard" {
				var perr error
				cores, perr = coreinstall.Profiles(*u)
				if perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "xray cores: %v\n", perr)
				}
			}
			cores = clientcfg.SortByRank(cores, func(p coreinstall.Profile) int { return clientcfg.CoreRank(p.Protocol) })
			nextCore := 0
			emitCores := func(limit int) {
				for nextCore < len(cores) && clientcfg.CoreRank(cores[nextCore].Protocol) <= limit {
					p := cores[nextCore]
					nextCore++
					link := p.Link
					if format == "standard" {
						if p.Protocol == "wireguard" {
							continue // a multi-line file has no one-line form
						}
					} else {
						var gerr error
						if link, gerr = clientcfg.GenerateCoreURI(p.Core, p.Link, cfg, clientcfg.URIOptions{Username: u.Username}); gerr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", p.Core, gerr)
							continue
						}
					}
					if annotated {
						fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", p.Core, p.Protocol, link)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), link)
					}
					count++
				}
			}
			sorted := clientcfg.SortByRank(cfg.Tunnels, func(t config.TunnelConfig) int { return clientcfg.TunnelRank(t.Transport) })
			for i := range sorted {
				t := &sorted[i]
				if !t.Enabled {
					continue
				}
				emitCores(clientcfg.TunnelRank(t.Transport))
				modes := []string{"dnstt"}
				if t.Transport == config.TransportDNSTT {
					modes = append(modes, "noizdns")
				}
				// NaiveProxy serves SOCKS and SSH clients: export both.
				for _, v := range clientcfg.Variants(t) {
					vt := *t
					vt.Backend = v.Backend
					vt.Tag = v.Tag
					if t.Transport == config.TransportNaive && cfg.GetBackend(v.Backend) == nil {
						continue
					}
					for _, mode := range modes {
						link, err := clientcfg.GeneratePortableURI(&vt, cfg.GetBackend(vt.Backend), cfg, clientcfg.URIOptions{Username: u.Username, Password: u.Password, ClientMode: mode}, format)
						if err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", vt.Tag, err)
							continue
						}
						if annotated {
							fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", vt.Tag, mode, link)
						} else {
							fmt.Fprintln(cmd.OutOrStdout(), link)
						}
						count++
					}
				}
			}
			emitCores(1 << 30)
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
