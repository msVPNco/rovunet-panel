package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/recoveryauth"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

type CarryPlan struct {
	Version                                                                              int
	Operation, Server, User, Account, OldPeriod, NewPeriod, Store, Directory, ConfigHash string
	Limit, Confirmed, Pending                                                            int64
	ExpiresAt                                                                            string
}
type carryJournal struct {
	User, PublicKey, Request, BeforeHash, AfterHash string
	Envelope                                        []byte
	Plan                                            CarryPlan
}
type carryResult struct{ Operation, Request, Account, Period, Outcome string }

var carryBarrier func(string) error

func carryStep(s string) error {
	if carryBarrier != nil {
		return carryBarrier(s)
	}
	return nil
}
func carryPending(cfg *config.Config) string { return cfg.Path() + ".carry.pending" }
func carryRoot(cfg *config.Config) string    { return cfg.Path() + ".carry" }
func CarryAccountPeriod(cfg *config.Config, envelope []byte, pinnedPublic string) error {
	var plan CarryPlan
	if e := recoveryauth.Verify(envelope, pinnedPublic, &plan); e != nil {
		return e
	}
	if plan.Limit <= 0 || plan.Version != 1 || !validPublicationID(plan.Operation, plan.Server, plan.Account, plan.OldPeriod, plan.NewPeriod) || plan.OldPeriod == plan.NewPeriod || plan.Pending != 0 {
		return quota.ErrConflict
	}
	return withPublicationLock(func() error {
		if e := cfg.RefreshIfChanged(); e != nil {
			return e
		}
		path := filepath.Join(carryRoot(cfg), plan.Operation+".receipt")
		if b, e := os.ReadFile(path); e == nil {
			var r carryResult
			if json.Unmarshal(b, &r) != nil || r.Request != digest(envelope) {
				return quota.ErrConflict
			}
			if r.Outcome != "done" {
				return errSuperseded
			}
			u := cfg.GetUser(plan.User)
			if u == nil || u.AccountID != r.Account || u.PeriodID != r.Period {
				return errSuperseded
			}
			return writeCurrentSubscriptions(cfg, discardOutput{})
		} else if !os.IsNotExist(e) {
			return e
		}
		if _, e := os.Stat(carryPending(cfg)); e == nil {
			b, e := os.ReadFile(carryPending(cfg))
			if e != nil {
				return e
			}
			var existing carryJournal
			if json.Unmarshal(b, &existing) != nil || existing.Request != digest(envelope) {
				return errors.New("another carry operation is pending")
			}
			return resumeCarry(cfg)
		} else if !os.IsNotExist(e) {
			return e
		}
		raw, e := os.ReadFile(cfg.Path())
		if e != nil {
			return e
		}
		if digest(raw) != plan.ConfigHash || cfg.ServerID != plan.Server {
			return quota.ErrConflict
		}
		u := cfg.GetUser(plan.User)
		if u == nil || u.AccountID != plan.Account || u.PeriodID != plan.OldPeriod || u.SQLiteMeter == nil || u.SQLiteMeter.Directory != plan.Directory || u.SQLiteMeter.Store != plan.Store || u.ExpiresAt != plan.ExpiresAt || u.QuotaBytes() != plan.Limit {
			return quota.ErrConflict
		}
		if u.Expired(time.Now()) {
			return errSuperseded
		}
		copy, e := config.LoadFrom(cfg.Path())
		if e != nil {
			return e
		}
		next := *u
		next.PeriodID = plan.NewPeriod
		copy.AddUser(next)
		after, e := json.MarshalIndent(copy, "", "  ")
		if e != nil {
			return e
		}
		j := carryJournal{User: plan.User, PublicKey: pinnedPublic, Request: digest(envelope), BeforeHash: plan.ConfigHash, AfterHash: digest(append(after, '\n')), Envelope: envelope, Plan: plan}
		if e = os.Mkdir(carryRoot(cfg), 0700); e != nil && !os.IsExist(e) {
			return e
		}
		// Original bytes are an immutable private archive; caller supplied external
		// identity/policy is never inferred from a database restored from elsewhere.
		if e = safefile.Write(filepath.Join(carryRoot(cfg), plan.Operation+".before.json"), raw, 0600); e != nil {
			return e
		}
		data, e := json.Marshal(j)
		if e != nil {
			return e
		}
		if e = safefile.Write(carryPending(cfg), data, 0600); e != nil {
			return e
		}
		if e = carryStep("prepared"); e != nil {
			return e
		}
		return resumeCarry(cfg)
	})
}
func resumeCarry(cfg *config.Config) error {
	path := carryPending(cfg)
	raw, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	var j carryJournal
	if json.Unmarshal(raw, &j) != nil || j.User != j.Plan.User || j.Request != digest(j.Envelope) {
		return errors.New("invalid carry journal")
	}
	var plan CarryPlan
	if e = recoveryauth.Verify(j.Envelope, j.PublicKey, &plan); e != nil {
		return e
	}
	a, _ := json.Marshal(plan)
	b, _ := json.Marshal(j.Plan)
	if string(a) != string(b) {
		return quota.ErrConflict
	}
	if e = cfg.RefreshIfChanged(); e != nil {
		return e
	}
	current, e := os.ReadFile(cfg.Path())
	if e != nil {
		return e
	}
	if cfg.ServerID != plan.Server {
		return quota.ErrConflict
	}
	ctx := context.Background()
	s, e := quota.Open(ctx, plan.Directory, plan.Server, plan.Store)
	if e != nil {
		return e
	}
	defer s.Close()
	if digest(current) == j.BeforeHash {
		e = cfg.CommitQuiescedFile(func(c *config.Config) error {
			amount, e := s.Read(ctx, quota.Key{Account: plan.Account, Period: plan.OldPeriod})
			if e != nil {
				return e
			}
			if amount.Confirmed != plan.Confirmed || amount.Pending != 0 {
				return errors.New("signed balance changed; keep account on hold")
			}
			var expiry *int64
			if plan.ExpiresAt != "" {
				t, e := time.Parse(time.RFC3339, plan.ExpiresAt)
				if e != nil {
					return errSuperseded
				}
				n := t.Unix()
				expiry = &n
			}
			if e = s.CarryPeriodWithExpiry(ctx, quota.Key{Account: plan.Account, Period: plan.OldPeriod}, plan.Operation, plan.NewPeriod, expiry); e != nil {
				return e
			}
			if e = carryStep("ledger-closed"); e != nil {
				return e
			}
			for i := range c.Users {
				if c.Users[i].Username == plan.User {
					c.Users[i].PeriodID = plan.NewPeriod
					return nil
				}
			}
			return quota.ErrConflict
		})
		if e != nil {
			cfg.Reload()
			return e
		}
		if e = carryStep("config-selected"); e != nil {
			return e
		}
	}
	current, e = os.ReadFile(cfg.Path())
	if e != nil {
		return e
	}
	outcome := "done"
	if digest(current) != j.AfterHash {
		outcome = "superseded"
	}
	// Suppress the account hold only while rendering the selected generation:
	// the journal remains durable until the receipt is written.
	if outcome == "done" {
		if e = writeCurrentSubscriptions(cfg, discardOutput{}); e != nil {
			return e
		}
		if e = carryStep("published"); e != nil {
			return e
		}
	}
	r := carryResult{plan.Operation, j.Request, plan.Account, plan.NewPeriod, outcome}
	data, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if e = safefile.Write(filepath.Join(carryRoot(cfg), plan.Operation+".receipt"), data, 0600); e != nil {
		return e
	}
	if e = carryStep("receipt"); e != nil {
		return e
	}
	if e = os.Remove(path); e != nil {
		return e
	}
	if e = safefile.SyncDir(filepath.Dir(path)); e != nil {
		return e
	}
	// Re-render after dropping the hold; never serve an old budget meanwhile.
	if outcome == "done" {
		return writeCurrentSubscriptions(cfg, discardOutput{})
	}
	return errSuperseded
}
