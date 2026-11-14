package cmd

import (
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/spf13/cobra"
	"path/filepath"
	"strconv"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "quota-preview-sample DIRECTORY SERVER STORE CONFIG CORE USER ACCOUNT PERIOD EPOCH UPLINK_BASE DOWNLINK_BASE", Hidden: true, Args: cobra.ExactArgs(11), PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, a []string) error {
		if filepath.Base(a[4]) != a[4] || a[4] == "." || a[4] == ".." || a[4] == "" {
			return errors.New("invalid configured core")
		}
		up, e := strconv.ParseInt(a[9], 10, 64)
		if e != nil {
			return e
		}
		down, e := strconv.ParseInt(a[10], 10, 64)
		if e != nil {
			return e
		}
		s, e := quota.Open(cmd.Context(), a[0], a[1], a[2])
		if e != nil {
			return e
		}
		defer s.Close()
		return coreinstall.SamplePreviewQuota(cmd.Context(), s, a[3], a[1], a[4], a[5], a[6], a[7], a[8], up, down)
	}})
}
