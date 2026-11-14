package cmd

import (
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func init() {
	command := &cobra.Command{Use: "recovery-checkpoint", Short: "Manage externally authenticated sealed recovery"}
	command.AddCommand(&cobra.Command{Use: "seal NEW-SNAPSHOT-DIRECTORY OPERATION-ID", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, a []string) error {
		c, e := config.Load()
		if e != nil {
			return e
		}
		h, e := handlers.SealRecoverySnapshot(cmd.Context(), c, a[0], a[1])
		if e == nil {
			fmt.Fprintln(cmd.OutOrStdout(), h)
		}
		return e
	}})
	command.AddCommand(&cobra.Command{Use: "new-target FRESH-DIRECTORY", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		id, e := handlers.InitializeRecoveryTarget(a[0])
		if e == nil {
			fmt.Fprintln(cmd.OutOrStdout(), id)
		}
		return e
	}})
	command.AddCommand(&cobra.Command{Use: "restore-target TARGET-DIRECTORY SNAPSHOT-DIRECTORY SIGNED-CLAIM PUBLIC-KEY-FILE", Args: cobra.ExactArgs(4), RunE: func(cmd *cobra.Command, a []string) error {
		b, e := os.ReadFile(a[2])
		if e != nil {
			return e
		}
		pub, e := os.ReadFile(a[3])
		if e != nil {
			return e
		}
		return handlers.RestoreSealedTarget(cmd.Context(), a[0], a[1], b, strings.TrimSpace(string(pub)))
	}})
	command.AddCommand(&cobra.Command{Use: "release SIGNED-RELEASE PUBLIC-KEY-FILE", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, a []string) error {
		c, e := config.Load()
		if e != nil {
			return e
		}
		b, e := os.ReadFile(a[0])
		if e != nil {
			return e
		}
		pub, e := os.ReadFile(a[1])
		if e != nil {
			return e
		}
		return handlers.ReleaseRecoverySeal(c, b, strings.TrimSpace(string(pub)))
	}})
	command.AddCommand(&cobra.Command{Use: "activate-empty TARGET-DIRECTORY SIGNED-CLAIM PUBLIC-KEY-FILE", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, a []string) error {
		b, e := os.ReadFile(a[1])
		if e != nil {
			return e
		}
		pub, e := os.ReadFile(a[2])
		if e != nil {
			return e
		}
		return handlers.ActivateRestoredInstallation(a[0], b, strings.TrimSpace(string(pub)))
	}})
	rootCmd.AddCommand(command)
}
