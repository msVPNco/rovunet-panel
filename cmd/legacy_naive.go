package cmd

import (
	"errors"
	"os"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "legacy-naive-runtime",
		Short: "Stage the pinned Naive runtime for standalone setup.sh",
		Args:  cobra.NoArgs,
		// Do not migrate panel state for this standalone, fixed-path operation.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() != 0 {
				return errors.New("root is required to install the standalone runtime")
			}
			if err := binary.InstallReviewedTransportAt("caddy-naive", "/usr/bin/caddy-naive"); err != nil {
				return err
			}
			cmd.Println("Pinned Naive runtime verified and staged; standalone configuration is unchanged.")
			return nil
		},
	})
}
