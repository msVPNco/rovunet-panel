package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/spf13/cobra"
)

func init() {
	c := &cobra.Command{Use: "backup-keygen", Short: "Create an owner-only backup identity; print only its public recipient", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("identity-file")
		if path == "" {
			return errors.New("--identity-file is required; keep this private key separately from backups")
		}
		id, err := age.GenerateX25519Identity()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return errors.New("cannot create backup identity; existing files are never overwritten")
		}
		good := false
		defer func() {
			f.Close()
			if !good {
				os.Remove(path)
			}
		}()
		if _, err = fmt.Fprintln(f, id.String()); err != nil {
			return errors.New("cannot write backup identity")
		}
		if err = f.Sync(); err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		if err = safefile.SyncDir(filepath.Dir(path)); err != nil {
			return err
		}
		good = true
		fmt.Fprintln(cmd.OutOrStdout(), id.Recipient().String())
		return nil
	}}
	c.Flags().String("identity-file", "", "New private key file (never printed or overwritten)")
	rootCmd.AddCommand(c)
}
