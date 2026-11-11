package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "core-awg-run <name>", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return coreinstall.RunAWG(ctx, args[0])
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "core-awg-sync <name>", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return coreinstall.SyncAWG(args[0]) }})
}
