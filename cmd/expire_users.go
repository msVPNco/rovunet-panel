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
		Short: "Retire users whose time or traffic limit is used up (run every 5 minutes by a timer)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			var out actions.OutputWriter = &actions.StdOutput{}
			gone, err := handlers.EnforceLimits(cfg, out, time.Now())
			for _, name := range gone {
				fmt.Printf("Retired user %s\n", name)
			}
			// A limited user's feed depends on which cores answer; refresh the
			// feeds each run (it writes only what changed), so a core that came
			// up after an update is added without waiting for a user change.
			if werr := handlers.WriteSubscriptions(cfg, out); werr != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "subscription files:", werr)
			}
			return err
		},
	})
}
