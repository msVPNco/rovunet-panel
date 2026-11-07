package cmd

import (
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

func init() {
	var o coreplan.Options
	var directory string
	c := &cobra.Command{Use: "core-plan", Short: "Create Xray server config and unique per-device client profiles; does not deploy", RunE: func(cmd *cobra.Command, args []string) error {
		bundle, err := coreplan.Generate(o)
		if err != nil {
			return err
		}
		if directory == "" {
			return fmt.Errorf("a new output directory is required")
		}
		if err := os.Mkdir(directory, 0700); err != nil {
			return fmt.Errorf("output directory must not already exist: %w", err)
		}
		if err := os.WriteFile(filepath.Join(directory, "server.json"), bundle.Server, 0600); err != nil {
			return err
		}
		for i, link := range bundle.Clients {
			if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("device-%03d.%s", i+1, bundle.Extension)), []byte(link+"\n"), 0600); err != nil {
				return err
			}
		}
		return os.WriteFile(filepath.Join(directory, "README.txt"), []byte("Generated configuration only. No service was installed. Check server.json with the pinned Xray core before deployment. Check TCP/UDP port conflicts and firewall policy. Never share a WireGuard device file between simultaneous devices. TLS requires a valid certificate for the SNI. Keep private keys private. Existing panel-managed tunnels were not modified.\n"), 0600)
	}}
	f := c.Flags()
	f.StringVar(&o.Protocol, "protocol", "", "reality, xhttp, hysteria2, wireguard")
	f.StringVar(&o.Host, "host", "", "Public endpoint hostname or IP")
	f.StringVar(&o.SNI, "sni", "", "Certificate name or REALITY server name")
	f.IntVar(&o.Port, "port", 8443, "Unoccupied TCP/UDP listener port")
	f.IntVar(&o.Devices, "devices", 1, "Distinct client credentials (1..1000)")
	f.StringVar(&o.Cert, "cert", "", "Server TLS certificate path")
	f.StringVar(&o.Key, "key", "", "Server TLS private key path")
	f.StringVar(&o.RealityTarget, "target", "", "REALITY TLS target host:port")
	f.StringVar(&directory, "output", "", "New private output directory")
	_ = c.MarkFlagRequired("protocol")
	_ = c.MarkFlagRequired("host")
	_ = c.MarkFlagRequired("output")
	rootCmd.AddCommand(c)
}
