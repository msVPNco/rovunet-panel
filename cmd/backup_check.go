package cmd

import (
	"fmt"
	"os"

	"github.com/msVPNco/rovunet-panel/internal/backup"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "backup-check <file>", Short: "Validate a private backup without restoring or printing credentials", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		info, err := os.Stat(args[0])
		if err != nil {
			return err
		}
		if info.Size() > 64<<20 {
			return fmt.Errorf("backup is too large")
		}
		raw, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		b, err := backup.Decode(string(raw))
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Backup valid: accounts=%d tunnels=%d cores=%d\n", len(b.Config.Users), len(b.Config.Tunnels), len(b.Cores))
		return nil
	}})
}
