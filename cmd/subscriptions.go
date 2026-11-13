package cmd

import (
	"fmt"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
)

// This maintenance command publishes static subscription files without
// migrating, recreating or restarting any service.
func subscriptionRefreshCommand(load func() (*config.Config, error)) *cobra.Command {
	return &cobra.Command{
		Use: "subscriptions-refresh", Short: "Refresh existing subscription URLs without restarting services",
		Args:              cobra.NoArgs,
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := load()
			if err != nil {
				return err
			}
			host := false
			for _, tunnel := range cfg.Tunnels {
				if tunnel.Enabled && tunnel.Transport == config.TransportNaive && tunnel.Domain != "" {
					host = true
					break
				}
			}
			if !host {
				return fmt.Errorf("no enabled subscription host is configured")
			}
			before := map[string]string{}
			for _, user := range cfg.Users {
				before[user.Username] = user.SubToken
			}
			if err := handlers.WriteSubscriptions(cfg, &actions.StdOutput{}); err != nil {
				return err
			}
			kept := 0
			for _, user := range cfg.Users {
				if token := before[user.Username]; token != "" {
					if token != user.SubToken {
						return fmt.Errorf("an existing subscription token changed unexpectedly")
					}
					kept++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Subscriptions refreshed: active_accounts=%d existing_links_kept=%d\n", len(cfg.Users), kept)
			return nil
		},
	}
}

func init() { rootCmd.AddCommand(subscriptionRefreshCommand(config.Load)) }

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "subscriptions-before-serve", Hidden: true, Args: cobra.NoArgs, PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(*cobra.Command, []string) error {
		cfg, e := config.Load()
		if e != nil {
			return e
		}
		return handlers.RecoverBeforeServing(cfg)
	}})
}
