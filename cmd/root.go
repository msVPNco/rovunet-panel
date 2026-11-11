package cmd

import (
	"fmt"
	"os"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/msVPNco/rovunet-panel/internal/menu"
	"github.com/msVPNco/rovunet-panel/internal/migrate"
	"github.com/msVPNco/rovunet-panel/internal/version"
	"github.com/spf13/cobra"

	// Register all actions
	_ "github.com/msVPNco/rovunet-panel/internal/actions"
)

var rootCmd = &cobra.Command{
	Use:   "rovunet",
	Short: "Unified server tunnel manager",
	Long:  "Rovunet Panel manages DNS tunnels (DNSTT, NoizDNS, Slipstream, VayDNS, Soar) and HTTPS proxies (NaiveProxy) with systemd services.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if v, _ := cmd.Flags().GetBool("version"); v {
			cmd.Println(version.String())
			return nil
		}
		// No args → launch interactive TUI
		cfg, err := loadOrDefaultConfig()
		return menu.Run(cfg, err)
	},
	// A server still on the pre-rename layout is migrated by the first
	// command run as root, before any command reads its config.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// post-update runs the migration itself, with its own error policy.
		if cmd.Name() != "post-update" && migrate.Needed() && os.Geteuid() == 0 {
			return migrate.Run(func(s string) { fmt.Println(s) })
		}
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Print version")

	// Wire dispatcher for menu → handlers (breaks import cycle)
	menu.Dispatcher = handlers.Dispatch

	// Register action-based subcommands
	registerActionCommands()

	// Hidden internal command: called by 'update' after replacing the binary
	// so that transport downloads and service regeneration use the new code.
	rootCmd.AddCommand(&cobra.Command{
		Use:    "post-update",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// A failing post-update makes a pre-rename updater fall back to
			// its own post-update code, which rebuilds the old-named
			// services from /etc/slipgate paths. That is only safe while
			// nothing was moved yet, so an error is returned only then.
			//
			// Before loading config: it may still be under the old name.
			if err := migrate.Run(func(s string) { fmt.Println(s) }); err != nil {
				fmt.Fprintln(os.Stderr, "Migration failed:", err)
				if migrate.Needed() {
					return err // old layout untouched: the old fallback is safe
				}
			}
			cfg, _ := loadOrDefaultConfig()
			ctx := &actions.Context{
				Args:   make(map[string]string),
				Output: &actions.StdOutput{},
				Config: cfg,
			}
			if err := handlers.HandlePostUpdate(ctx); err != nil {
				fmt.Fprintln(os.Stderr, "Warning: post-update:", err)
			}
			// Units written by v1.8.0/v1.8.1 from the removed old command's
			// path fail with 203/EXEC; point them at the installed binary.
			if fixed, err := migrate.RepairUnits(); err != nil {
				fmt.Fprintln(os.Stderr, "Warning: repairing units:", err)
			} else if len(fixed) > 0 {
				migrate.Restart(fixed)
				fmt.Printf("Repaired %d service(s) that still ran the old command\n", len(fixed))
			}
			// Cores that serve a TLS certificate must restart after it renews.
			if err := coreinstall.EnsureCertTimer(); err != nil {
				fmt.Fprintln(os.Stderr, "Warning: certificate renewal timer:", err)
			}
			// Users with an expiry need the hourly timer that removes them.
			if cfg != nil {
				if err := handlers.EnsureExpiryTimer(cfg); err != nil {
					fmt.Fprintln(os.Stderr, "Warning: user expiry timer:", err)
				}
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	})
}

func registerActionCommands() {
	groups := map[string]*cobra.Command{
		"tunnel": {
			Use:   "tunnel",
			Short: "Manage tunnels",
		},
		"router": {
			Use:   "router",
			Short: "Manage DNS router",
		},
		"config": {
			Use:   "config",
			Short: "Import/export configuration",
		},
	}

	for _, g := range groups {
		rootCmd.AddCommand(g)
	}

	// System-level commands go directly on root
	systemActions := map[string]string{
		actions.QuickWizard:     "wizard",
		actions.SystemInstall:   "install",
		actions.SystemUninstall: "uninstall",
		actions.SystemUpdate:    "update",
		actions.SystemRestart:   "restart",
		actions.SystemUsers:     "users",
		actions.SystemStats:     "stats",
		actions.SystemDiag:      "diag",
		actions.SystemMTU:       "mtu",
		actions.SystemBackup:    "backup",
		actions.SystemRestore:   "restore",
	}

	for id, use := range systemActions {
		a, ok := actions.Get(id)
		if !ok {
			continue
		}
		rootCmd.AddCommand(actionToCommand(a, use))
	}

	// Grouped commands
	tunnelActions := map[string]string{
		actions.TunnelAdd:    "add",
		actions.TunnelRemove: "remove",
		actions.TunnelStart:  "start",
		actions.TunnelStop:   "stop",
		actions.TunnelShare:  "share",
		actions.TunnelStatus: "status",
		actions.TunnelLogs:   "logs",
		actions.TunnelEdit:   "edit",
	}
	for id, use := range tunnelActions {
		a, ok := actions.Get(id)
		if !ok {
			continue
		}
		groups["tunnel"].AddCommand(actionToCommand(a, use))
	}

	routerActions := map[string]string{
		actions.RouterStatus: "status",
		actions.RouterMode:   "mode",
		actions.RouterSwitch: "switch",
	}
	for id, use := range routerActions {
		a, ok := actions.Get(id)
		if !ok {
			continue
		}
		groups["router"].AddCommand(actionToCommand(a, use))
	}

	configActions := map[string]string{
		actions.ConfigExport:  "export",
		actions.ConfigImport:  "import",
		actions.ConfigAddress: "address",
	}
	for id, use := range configActions {
		a, ok := actions.Get(id)
		if !ok {
			continue
		}
		groups["config"].AddCommand(actionToCommand(a, use))
	}
}

func loadOrDefaultConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return config.Default(), nil
		}
		return config.Default(), err
	}
	return cfg, nil
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
