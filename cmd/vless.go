package cmd

import (
	"context"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/usage"
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
			meter := usage.NewCounter("vless")
			initial, readErr := usage.TotalsChecked()
			baseline := meter.Snapshot()
			quotaUsed := func(user string) int64 {
				if readErr != nil {
					return 1<<63 - 1
				}
				return initial[user] + (meter.Value(user) - baseline[user])
			}
			var store *quota.Store
			var server string
			dir, _ := cmd.Flags().GetString("preview-quota-directory")
			if dir != "" {
				if !strings.HasPrefix(listen, "127.0.0.1:") {
					return errors.New("quota preview requires isolated loopback listener")
				}
				cfg, e := config.LoadFrom(users)
				if e != nil {
					return e
				}
				server = cfg.ServerID
				storeID, _ := cmd.Flags().GetString("preview-quota-id")
				store, e = quota.Open(cmd.Context(), dir, server, storeID)
				if e != nil {
					return e
				}
				defer store.Close()
				quotaUsed = nil
			}
			srv, err := proxy.NewVLESSServer(proxy.VLESSOptions{
				QuotaUsed: quotaUsed, QuotaStore: store, QuotaServer: server,
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

			proxy.Meter = meter
			flushed := make(chan struct{})
			go func() { defer close(flushed); meter.Run(ctx) }()
			serveErr := srv.ListenAndServe(ctx)
			stop()
			<-flushed
			if err := meter.Flush(); err != nil {
				return err
			}
			return serveErr
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	serveCmd.Flags().String("preview-quota-directory", "", "Private SQL canary")
	serveCmd.Flags().String("preview-quota-id", "", "Expected store identity")
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
