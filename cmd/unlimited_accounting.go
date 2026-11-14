package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/spf13/cobra"
)

func init() {
	root := &cobra.Command{Use: "unlimited-accounting", Short: "Inspect unlimited accounting and manage a reviewed single-account migration", PersistentPreRunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(&cobra.Command{Use: "plan CONFIG NEW-PRIVATE-DIRECTORY", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, a []string) error {
		c, e := config.LoadFrom(a[0])
		if e != nil {
			return e
		}
		return handlers.PlanUnlimitedMigration(c, a[1])
	}})
	root.AddCommand(&cobra.Command{Use: "apply PRIVATE-DIRECTORY", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, a []string) error { return handlers.ApplyUnlimitedMigration(a[0]) }})
	root.AddCommand(&cobra.Command{Use: "observe [CONFIG]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		p := config.DefaultConfigFile
		if len(a) > 0 {
			p = a[0]
		}
		c, e := config.LoadFrom(p)
		if e != nil {
			return e
		}
		if e = coreinstall.CollectUsage(); e != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "Engine collection partial; only structurally valid recorded lower bounds will be observed")
		}
		return handlers.ObserveUnlimitedAccounts(c)
	}})
	var username string
	status := &cobra.Command{Use: "status [CONFIG]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		p := config.DefaultConfigFile
		if len(a) > 0 {
			p = a[0]
		}
		c, e := config.LoadFrom(p)
		if e != nil {
			return e
		}
		matches := 0
		for _, u := range c.Users {
			if u.SQLiteMeter != nil && u.QuotaGB == 0 {
				matches++
			}
		}
		if matches > 1 && username == "" {
			return fmt.Errorf("multiple unlimited accounts; select one with --user NAME")
		}
		for _, u := range c.Users {
			if username != "" && u.Username != username {
				continue
			}
			if u.SQLiteMeter == nil || u.QuotaGB != 0 {
				continue
			}
			b := u.SQLiteMeter
			s, e := quota.Open(cmd.Context(), b.Directory, c.ServerID, b.Store)
			if e != nil {
				return e
			}
			m, e := s.Metadata(context.Background(), quota.Key{Account: u.AccountID, Period: u.PeriodID})
			s.Close()
			if e != nil {
				return e
			}
			if m.LimitKind != "unlimited" || m.State != "active" || m.Pending != 0 {
				return quota.ErrConflict
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"backend": "sqlite", "limit_kind": m.LimitKind, "confirmed_bytes": m.Confirmed, "pending_bytes": m.Pending, "coverage": "partial", "measurement": "recorded_lower_bound", "period_preserved": true})
		}
		return fmt.Errorf("no matching unlimited SQLite account")
	}}
	status.Flags().StringVar(&username, "user", "", "Account name on a multi-account server")
	root.AddCommand(status)
	root.AddCommand(&cobra.Command{Use: "collect", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, a []string) error {
		if e := coreinstall.CollectUsage(); e != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "Engine collection partial; recorded usage retained")
		}
		return nil
	}})
	rootCmd.AddCommand(root)
}
