package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/msVPNco/rovunet-panel/internal/backup"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
)

func init() {
	check := &cobra.Command{Use: "backup-check <file>", Short: "Validate a private backup without restoring or printing credentials", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		info, err := os.Stat(args[0])
		if err != nil {
			return err
		}
		if info.Size() > 64<<20 {
			return fmt.Errorf("backup is too large")
		}
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		raw, err := io.ReadAll(io.LimitReader(f, backup.MaxEncodedSize+1))
		if err != nil {
			return err
		}
		if len(raw) > backup.MaxEncodedSize {
			return fmt.Errorf("backup is too large")
		}
		identity, _ := cmd.Flags().GetString("identity-file")
		passfile, _ := cmd.Flags().GetString("passphrase-file")
		b, err := handlers.DecodePrivateBackup(string(raw), identity, passfile)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Backup valid: accounts=%d tunnels=%d cores=%d\n", len(b.Config.Users), len(b.Config.Tunnels), len(b.Cores))
		return nil
	}}
	check.Flags().String("identity-file", "", "Owner-only age private identity file")
	check.Flags().String("passphrase-file", "", "Owner-only passphrase file")
	rootCmd.AddCommand(check)
}
