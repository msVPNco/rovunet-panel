package cmd

import (
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "quota-preview-snapshot CONFIG NEW-DIRECTORY OPERATION-ID", Hidden: true, Args: cobra.ExactArgs(3), PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, a []string) error {
		c, e := config.LoadFrom(a[0])
		if e != nil {
			return e
		}
		hash, e := handlers.CreateCoherentSnapshot(cmd.Context(), c, a[1], a[2])
		if e != nil {
			return e
		}
		fmt.Fprintln(cmd.OutOrStdout(), hash)
		return nil
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "quota-preview-reconcile CONFIG SNAPSHOT-DIRECTORY TRUSTED-MANIFEST-SHA256", Hidden: true, Args: cobra.ExactArgs(3), PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, a []string) error {
		c, e := config.LoadFrom(a[0])
		if e != nil {
			return e
		}
		return handlers.ReconcileCoherentSnapshot(cmd.Context(), c, a[1], a[2])
	}})
}
