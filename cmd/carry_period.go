package cmd

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "carry-period SIGNED-MANIFEST EXTERNALLY-PINNED-PUBLIC-KEY-FILE", Args: cobra.ExactArgs(2), RunE: func(*cobra.Command, []string) error { return nil }})
	command := rootCmd.Commands()
	for _, c := range command {
		if c.Name() == "carry-period" {
			c.RunE = func(cmd *cobra.Command, a []string) error {
				cfg, e := config.Load()
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
				return handlers.CarryAccountPeriod(cfg, b, strings.TrimSpace(string(pub)))
			}
		}
	}
}
