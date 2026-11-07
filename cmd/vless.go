package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/spf13/cobra"
)

func init() {
	vlessCmd := &cobra.Command{
		Use:   "vless",
		Short: "Built-in VLESS over WebSocket server",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the VLESS over WebSocket server (used by systemd)",
		RunE: func(cmd *cobra.Command, args []string) error {
			listen, _ := cmd.Flags().GetString("listen")
			path, _ := cmd.Flags().GetString("path")
			cert, _ := cmd.Flags().GetString("cert")
			key, _ := cmd.Flags().GetString("key")
			secret, _ := cmd.Flags().GetString("secret-file")
			users, _ := cmd.Flags().GetString("users-file")
			blockPorts, _ := cmd.Flags().GetString("block-ports")
			verbose, _ := cmd.Flags().GetBool("verbose")
			allowPrivate, _ := cmd.Flags().GetBool("allow-private")

			ports, err := proxy.ParsePortList(blockPorts)
			if err != nil {
				return err
			}
			srv, err := proxy.NewVLESSServer(proxy.VLESSOptions{
				Listen:       listen,
				Path:         path,
				CertFile:     cert,
				KeyFile:      key,
				SecretFile:   secret,
				UsersFile:    users,
				BlockPorts:   ports,
				AllowPrivate: allowPrivate,
				Verbose:      verbose,
			})
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// SIGHUP reloads the users at once (they are also picked up
			// within seconds of the users file changing).
			hup := make(chan os.Signal, 1)
			signal.Notify(hup, syscall.SIGHUP)
			go func() {
				for range hup {
					if n, err := srv.ReloadUsers(); err != nil {
						log.Printf("%v", err)
					} else {
						log.Printf("vless: users reloaded (%d)", n)
					}
				}
			}()

			return srv.ListenAndServe(ctx)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	serveCmd.Flags().String("listen", "0.0.0.0:443", "Listen address")
	serveCmd.Flags().String("path", "/", "WebSocket path")
	serveCmd.Flags().String("cert", "", "TLS certificate file (omit for plain HTTP)")
	serveCmd.Flags().String("key", "", "TLS private key file")
	serveCmd.Flags().String("secret-file", "", "Secret that derives each user's UUID")
	serveCmd.Flags().String("users-file", config.DefaultConfigFile, "Panel config holding the users")
	serveCmd.Flags().String("block-ports", config.DefaultVLESSBlockPorts, `Destination ports to refuse (comma-separated, or "none")`)
	serveCmd.Flags().Bool("verbose", false, "Log each stream and rejected request")
	serveCmd.Flags().Bool("allow-private", false, "Let users reach private and loopback addresses (testing only)")
	_ = serveCmd.Flags().MarkHidden("allow-private")

	vlessCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(vlessCmd)
}
