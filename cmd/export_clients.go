package cmd

import (
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	var user, format, client string
	var annotated, all bool
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
			profiles, exportErr := clientexport.Build(cfg, *u, cfg.Tunnels, clientexport.Options{Format: format, Client: client, All: all})
			for _, p := range profiles {
				if annotated {
					fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", p.ID, p.Protocol, p.Link)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), p.Link)
				}
			}
			if exportErr != nil {
				return exportErr
			}
			if len(profiles) == 0 {
				return fmt.Errorf("no compatible enabled profiles exported for this account")
			}
			return nil
		},
	}
	command.Flags().StringVar(&user, "user", "", "Existing managed username")
	command.Flags().StringVar(&format, "format", "auto", "auto, standard, rovunet, veyra (compatibility), or legacy")
	command.Flags().StringVar(&client, "client", "android", "android or windows (capabilities of the released client)")
	command.Flags().BoolVar(&annotated, "annotated", false, "Prefix each line with its tunnel tag and client mode, e.g. \"tag (dnstt): rovunet://...\". Leave off for scripts, which parse one bare link per line")
	command.Flags().BoolVar(&all, "all", false, "Also export plain (unencrypted) SOCKS5 configs, left out by default")
	_ = command.MarkFlagRequired("user")
	rootCmd.AddCommand(command)
}
