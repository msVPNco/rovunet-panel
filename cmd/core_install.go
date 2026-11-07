package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/spf13/cobra"
)

func init() {
	var o coreplan.Options
	var name string
	install := &cobra.Command{
		Use:   "core-install",
		Short: "Install an Xray core (REALITY, XHTTP, Hysteria2, WireGuard) as a managed service",
		RunE: func(cmd *cobra.Command, args []string) error {
			var dir string
			var err error
			perDevice := cmd.Flags().Changed("devices")
			if perDevice {
				dir, err = coreinstall.Install(name, o)
			} else {
				cfg, _ := loadOrDefaultConfig()
				dir, err = coreinstall.InstallForUsers(name, o, cfg.Users)
			}
			if dir == "" && err != nil {
				return err
			}
			if err != nil {
				fmt.Println("Warning:", err)
			}
			fmt.Printf("Core %q is running as %s\n", name, coreinstall.ServiceName(name))
			if perDevice {
				fmt.Printf("Per-device client configs: %s\n", filepath.Join(dir, "clients"))
				fmt.Println("Give each device its own file; never share one WireGuard file between devices.")
			} else {
				fmt.Println("Every panel user has a configuration on it: 'rovunet users configs' prints them with the rest.")
			}
			return nil
		},
	}
	f := install.Flags()
	f.StringVar(&name, "name", "", "Core name (a-z, 0-9, '-')")
	f.StringVar(&o.Protocol, "protocol", "", "reality, xhttp, hysteria2, wireguard")
	f.StringVar(&o.Host, "host", "", "Public endpoint hostname or IP")
	f.StringVar(&o.SNI, "sni", "", "Certificate name or REALITY server name")
	f.IntVar(&o.Port, "port", 8443, "Listener port (must be free)")
	f.IntVar(&o.Devices, "devices", 1, "Per-device mode: this many standalone credentials instead of one per panel user (1..1000)")
	f.StringVar(&o.Cert, "cert", "", "Server TLS certificate path (xhttp, hysteria2)")
	f.StringVar(&o.Key, "key", "", "Server TLS private key path (xhttp, hysteria2)")
	f.StringVar(&o.RealityTarget, "target", "", "REALITY TLS target host:port")
	for _, r := range []string{"name", "protocol", "host"} {
		_ = install.MarkFlagRequired(r)
	}

	remove := &cobra.Command{
		Use:   "core-remove <name>",
		Short: "Stop and delete an installed Xray core",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := coreinstall.Remove(args[0]); err != nil {
				return err
			}
			fmt.Printf("Core %q removed\n", args[0])
			return nil
		},
	}

	list := &cobra.Command{
		Use:   "core-list",
		Short: "List installed Xray cores",
		RunE: func(cmd *cobra.Command, args []string) error {
			cores, err := coreinstall.List()
			if err != nil {
				return err
			}
			if len(cores) == 0 {
				fmt.Println("No cores installed")
			}
			for _, c := range cores {
				fmt.Printf("%s\t%s\t%d/%s\n", c.Name, c.Protocol, c.Port, c.Proto)
			}
			return nil
		},
	}
	certs := &cobra.Command{
		Use:   "core-certs",
		Short: "Restart Xray cores whose TLS certificate was renewed (run daily by a timer)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			restarted, err := coreinstall.RestartIfCertChanged()
			for _, name := range restarted {
				fmt.Printf("Restarted core %q: its certificate was renewed\n", name)
			}
			return err
		},
	}
	rootCmd.AddCommand(install, remove, list, certs)
}
