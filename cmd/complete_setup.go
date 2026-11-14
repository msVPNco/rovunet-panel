package cmd

import (
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "core-sync-users", Short: "Repair missing per-account core profiles without changing subscription URLs", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, e := config.Load()
		if e != nil {
			return e
		}
		if e = coreinstall.RepairMissingProfiles(cfg.Users); e != nil {
			return e
		}
		return handlers.WriteSubscriptions(cfg, &actions.StdOutput{})
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "finite-accounting-migrate", Short: "Resume durable finite-account accounting cutover without changing subscription URLs", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, e := config.Load()
		if e != nil {
			return e
		}
		return handlers.MigrateFiniteAccounting(cfg, &actions.StdOutput{})
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "complete-setup", Short: "Resume automatic core setup and refresh existing subscriptions", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return handlers.HandleCompleteSetup(&actions.Context{Config: cfg, Output: &actions.StdOutput{}, Args: map[string]string{}})
		},
	})
}
