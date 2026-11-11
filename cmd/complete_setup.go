package cmd

import (
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
)

func init() {
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
