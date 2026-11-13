package cmd

import (
	"fmt"
	"strconv"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/spf13/cobra"
)

func refreshCoreSubscriptions() error {
	cfg, err := loadOrDefaultConfig()
	if err != nil {
		return err
	}
	return handlers.WriteSubscriptions(cfg, &actions.StdOutput{})
}

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "core-hy2-obfs <name> <on|off>", Short: "Switch one Hy2 core's Salamander mode with private recovery and rollback", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if args[1] != "on" && args[1] != "off" {
			return fmt.Errorf("mode must be on or off")
		}
		cfg, err := loadOrDefaultConfig()
		if err != nil {
			return err
		}
		backup, err := coreinstall.ChangeHysteriaObfuscation(args[0], args[1] == "on", cfg.Users)
		if backup != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "Private core recovery files:", backup)
		}
		if err != nil {
			return err
		}
		if err := refreshCoreSubscriptions(); err != nil {
			return fmt.Errorf("core mode applied; retry subscriptions-refresh: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Hy2 mode applied. Existing subscription URLs are retained; clients must refresh their saved profiles.")
		return nil
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "core-confirm <name>", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return coreinstall.Confirm(args[0]) }})
	rootCmd.AddCommand(&cobra.Command{Use: "core-prepare <name>", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadOrDefaultConfig()
		if err != nil {
			return err
		}
		return coreinstall.Prepare(args[0], cfg.Users)
	}})
	for _, action := range []string{"start", "stop"} {
		action := action
		rootCmd.AddCommand(&cobra.Command{Use: "core-" + action + " <name>", Short: action + " a managed protocol", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if action == "start" {
				err = coreinstall.Start(args[0])
			} else {
				err = coreinstall.Stop(args[0])
			}
			if err != nil {
				return err
			}
			return refreshCoreSubscriptions()
		}})
	}
	rootCmd.AddCommand(&cobra.Command{Use: "core-port <name> <port>", Short: "Change one managed protocol's port with rollback", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		cfg, err := loadOrDefaultConfig()
		if err != nil {
			return err
		}
		if err := coreinstall.ChangePort(args[0], port, cfg.Users); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Port changed; exclusive owned rules were released, while preexisting/shared rules were preserved.")
		return refreshCoreSubscriptions()
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "core-status <name>", Short: "Show a managed protocol's state without credentials", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := coreinstall.ValidateName(args[0]); err != nil {
			return err
		}
		m, err := coreinstall.ReadMeta(args[0])
		if err != nil {
			return err
		}
		active, _ := service.Status(coreinstall.ServiceName(args[0]))
		fmt.Fprintf(cmd.OutOrStdout(), "%s protocol=%s port=%d transport=%s state=%s\n", m.Name, m.Protocol, m.Port, m.Proto, active)
		return nil
	}})
}
