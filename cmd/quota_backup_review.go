package cmd

import (
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/spf13/cobra"
)

// Both commands operate on explicitly named lab stores. Merge requires the
// operator to fence writers and obtain a digest from an authenticated manifest.
func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "quota-preview-backup DIRECTORY SERVER STORE FRESH_DEST", Hidden: true, Args: cobra.ExactArgs(4), PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, a []string) error {
		s, e := quota.Open(cmd.Context(), a[0], a[1], a[2])
		if e != nil {
			return e
		}
		defer s.Close()
		return s.Backup(cmd.Context(), a[3])
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "quota-preview-merge DIRECTORY SERVER STORE ACCOUNT PERIOD SNAPSHOT MANIFEST_DIGEST", Hidden: true, Args: cobra.ExactArgs(7), PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, a []string) error {
		s, e := quota.Open(cmd.Context(), a[0], a[1], a[2])
		if e != nil {
			return e
		}
		defer s.Close()
		return s.MergeBackup(cmd.Context(), quota.Key{Account: a[3], Period: a[4]}, a[5], a[6])
	}})
}
