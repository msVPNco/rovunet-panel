package cmd

import (
	"fmt"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "expire-users",
		Short: "Remove users whose expiry date has passed (run hourly by a timer)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			var out actions.OutputWriter = &actions.StdOutput{}
			gone, err := handlers.ExpireUsers(cfg, out, time.Now())
			for _, name := range gone {
				fmt.Printf("Removed expired user %q\n", name)
			}
			return err
		},
	})
}
