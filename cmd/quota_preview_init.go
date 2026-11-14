package cmd

import (
	"encoding/json"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/spf13/cobra"
	"io"
	"os"
)

type labOpening struct {
	Account, Period, Origin, Proof string
	Limit, Confirmed               *int64
}
type labPlan struct {
	Server, Store string
	Openings      []labOpening
}

func init() {
	command := &cobra.Command{Use: "quota-preview-init MANIFEST FRESH-DIRECTORY", Hidden: true, Args: cobra.ExactArgs(2), PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		f, e := os.Open(args[0])
		if e != nil {
			return e
		}
		defer f.Close()
		d := json.NewDecoder(f)
		d.DisallowUnknownFields()
		var p labPlan
		if e = d.Decode(&p); e != nil {
			return e
		}
		if e = d.Decode(new(any)); e != io.EOF {
			return errors.New("invalid trailing manifest")
		}
		if len(p.Openings) == 0 {
			return errors.New("missing confirmed opening balances")
		}
		seen := map[string]bool{}
		for _, r := range p.Openings {
			if r.Account == "" || r.Period == "" || r.Limit == nil || r.Confirmed == nil || *r.Limit < 0 || *r.Confirmed < 0 || r.Origin == "" || r.Proof == "" || seen[r.Account] {
				return errors.New("invalid opening")
			}
			seen[r.Account] = true
		}
		var s *quota.Store
		if resume, _ := cmd.Flags().GetBool("resume"); resume {
			s, e = quota.ResumeBootstrap(cmd.Context(), args[1], p.Server, p.Store)
		} else {
			s, e = quota.Create(cmd.Context(), args[1], p.Server, p.Store)
		}
		if e != nil {
			return e
		}
		defer s.Close()
		for _, r := range p.Openings {
			k := quota.Key{Account: r.Account, Period: r.Period}
			if e = s.NewPeriod(cmd.Context(), k, *r.Limit); e != nil {
				return e
			}
			if e = s.ImportOpening(cmd.Context(), quota.Checkpoint{Server: p.Server, Store: p.Store, Account: r.Account, Period: r.Period, ID: r.Origin, OpeningOrigin: r.Origin, Proof: r.Proof, Opening: *r.Confirmed}); e != nil {
				return e
			}
		}
		return s.FinishBootstrap(args[1])
	}}
	command.Flags().Bool("resume", false, "Explicitly replay the original authenticated bootstrap manifest")
	rootCmd.AddCommand(command)
}
