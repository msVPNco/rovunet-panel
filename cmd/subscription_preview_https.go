package cmd

import (
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/handlers"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/spf13/cobra"
	"net"
	"net/http"
	"time"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "subscription-preview-https LOOPBACK-LISTEN CONFIG CERT KEY [QUOTA_DIR SERVER STORE]", Args: cobra.RangeArgs(4, 7), Hidden: true, PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, a []string) error {
		host, _, e := net.SplitHostPort(a[0])
		if e != nil || !net.ParseIP(host).IsLoopback() {
			return errors.New("local preview requires loopback")
		}
		c, e := config.LoadFrom(a[1])
		if e != nil {
			return e
		}
		if e = handlers.ValidateV2Authority(c); e != nil {
			return e
		}
		var store *quota.Store
		var server string
		if len(a) != 4 && len(a) != 7 {
			return errors.New("provide all three quota arguments")
		}
		if len(a) == 7 {
			server = a[5]
			store, e = quota.Open(cmd.Context(), a[4], server, a[6])
			if e != nil {
				return e
			}
			defer store.Close()
		}
		s := http.Server{Addr: a[0], Handler: handlers.SubscriptionMetadataHandler(handlers.ResolveV2FromConfig(a[1], store, server)), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second}
		return s.ListenAndServeTLS(a[2], a[3])
	}})
}
