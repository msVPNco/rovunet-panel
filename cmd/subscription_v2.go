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
	command := &cobra.Command{Use: "subscription-v2", Short: "Manage the opt-in local subscription v2 renderer"}
	command.AddCommand(&cobra.Command{Use: "serve", Hidden: true, Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		ctx, cancel := handlers.RendererSignalContext()
		defer cancel()
		return handlers.ServeSubscriptionV2(ctx, config.DefaultConfigFile)
	}})
	for _, action := range []string{"enable", "disable", "refresh"} {
		a := action
		command.AddCommand(&cobra.Command{Use: a, Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
			c, e := config.Load()
			if e != nil {
				return e
			}
			if a == "refresh" {
				return handlers.ApplyV2Deployment(c)
			}
			return handlers.SetV2Deployment(c, a == "enable")
		}})
	}

	command.AddCommand(&cobra.Command{Use: "initialize [CONFIG]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		path := config.DefaultConfigFile
		if len(a) == 1 {
			path = a[0]
		}
		c, e := config.LoadFrom(path)
		if e != nil {
			return e
		}
		return handlers.InitializeV2Authority(c)
	}})
	command.AddCommand(&cobra.Command{Use: "recover-commit", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		c, e := config.Load()
		if e != nil {
			return e
		}
		return handlers.RecoverV2Revision(c)
	}})
	command.AddCommand(&cobra.Command{Use: "checkpoint NEW-PRIVATE-DIRECTORY", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		c, e := config.Load()
		if e != nil {
			return e
		}
		h, e := handlers.CheckpointV2Authority(c, a[0])
		if e == nil {
			fmt.Fprintln(cmd.OutOrStdout(), h)
		}
		return e
	}})
	command.AddCommand(&cobra.Command{Use: "repair-authority CHECKPOINT EXTERNAL-LATEST-MANIFEST-SHA256", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, a []string) error {
		c, e := config.Load()
		if e != nil {
			return e
		}
		return handlers.RepairV2Authority(c, a[0], a[1])
	}})
	command.AddCommand(&cobra.Command{Use: "adopt-legacy SIGNED-MANIFEST PINNED-PUBLIC-KEY-FILE", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, a []string) error {
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
		return handlers.AdoptLegacyV2Authority(c, b, strings.TrimSpace(string(pub)))
	}})
	rootCmd.AddCommand(command)
}
