package cmd

import (
	"fmt"
	"github.com/anonvector/slipgate/internal/clientcfg"
	"github.com/anonvector/slipgate/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	var user, format string
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
					fmt.Fprintln(cmd.OutOrStdout(), link)
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
	_ = command.MarkFlagRequired("user")
	rootCmd.AddCommand(command)
}
