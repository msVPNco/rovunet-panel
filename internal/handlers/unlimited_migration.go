package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/usage"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var unlimitedMigrationBarrier func(string) error

func unlimitedStep(p string) error {
	if unlimitedMigrationBarrier != nil {
		return unlimitedMigrationBarrier(p)
	}
	return nil
}

type UnlimitedPlan struct {
	Version                                                                           int
	Operation, ConfigPath, Server, Account, Period, User, Store, Directory, UsageHash string
	Before, After                                                                     []byte
	Opening, Watermark                                                                int64
}

func unlimitedUser(c *config.Config) (config.UserConfig, error) {
	if len(c.Users) != 1 || len(c.Archived) != 0 {
		return config.UserConfig{}, errors.New("single active account required")
	}
	u := c.Users[0]
	if u.QuotaGB != 0 || u.Expired(time.Now()) || !validPublicationID(c.ServerID, u.AccountID, u.PeriodID, u.SubToken) || u.UsageBase < 0 {
		return u, quota.ErrConflict
	}
	return u, nil
}
func PlanUnlimitedMigration(c *config.Config, dir string) error {
	return withPublicationLock(func() error {
		u, e := unlimitedUser(c)
		if e != nil {
			return e
		}
		if u.SQLiteMeter != nil || c.SubscriptionV2 || c.V2Authority != nil || config.AccountOnHold(c.Path(), u.Username) || c.RecoverySealed {
			return quota.ErrConflict
		}
		if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
			return quota.ErrConflict
		}
		if _, err := os.Lstat(c.Path() + ".unlimited-migration.anchor"); !os.IsNotExist(err) {
			return errors.New("prior migration identity exists")
		}
		snap, e := usage.RecordedSnapshot(u.Username)
		if e != nil {
			return e
		}
		if snap.Total < u.UsageBase {
			return errors.New("recorded counters below usage baseline; reconciliation required")
		}
		raw, e := os.ReadFile(c.Path())
		if e != nil {
			return e
		}
		p := UnlimitedPlan{Version: 1, Operation: config.NewID(), ConfigPath: c.Path(), Server: c.ServerID, Account: u.AccountID, Period: u.PeriodID, User: u.Username, Store: config.NewID(), Directory: filepath.Join(dir, "db"), Before: raw, Opening: snap.Total - u.UsageBase, Watermark: snap.Total, UsageHash: snap.Fingerprint}
		// Preserve unknown config fields too: the only semantic edit is this binding.
		var top map[string]json.RawMessage
		if e = json.Unmarshal(raw, &top); e != nil {
			return e
		}
		var users []map[string]json.RawMessage
		if e = json.Unmarshal(top["users"], &users); e != nil || len(users) != 1 {
			return quota.ErrConflict
		}
		users[0]["sqlite_meter"], e = json.Marshal(config.SQLiteBinding{Directory: p.Directory, Store: p.Store})
		if e != nil {
			return e
		}
		top["users"], e = json.Marshal(users)
		if e != nil {
			return e
		}
		p.After, e = json.MarshalIndent(top, "", "  ")
		if e != nil {
			return e
		}
		p.After = append(p.After, '\n')
		if e = os.Mkdir(dir, 0700); e != nil {
			return e
		}
		body, e := json.Marshal(p)
		if e != nil {
			return e
		}
		if e = safefile.Write(filepath.Join(dir, "plan.json"), body, 0600); e != nil {
			return e
		}
		return safefile.SyncDir(filepath.Dir(dir))
	})
}
func loadUnlimitedPlan(dir string) (UnlimitedPlan, error) {
	var p UnlimitedPlan
	info, e := os.Lstat(dir)
	if e != nil {
		return p, e
	}
	if !ownedMigrationPath(info) || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return p, quota.ErrConflict
	}
	file := filepath.Join(dir, "plan.json")
	info, e = os.Lstat(file)
	if e != nil {
		return p, e
	}
	if !ownedMigrationPath(info) || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return p, quota.ErrConflict
	}
	b, e := os.ReadFile(file)
	if e != nil {
		return p, e
	}
	if e = json.Unmarshal(b, &p); e != nil {
		return p, e
	}
	if p.Version != 1 || !validPublicationID(p.Operation, p.Server, p.Account, p.Period, p.Store) || p.Directory != filepath.Join(dir, "db") || !filepath.IsAbs(p.ConfigPath) || p.Opening < 0 || p.Watermark < p.Opening {
		return p, quota.ErrConflict
	}
	var before, after map[string]json.RawMessage
	if json.Unmarshal(p.Before, &before) != nil || json.Unmarshal(p.After, &after) != nil {
		return p, quota.ErrConflict
	}
	var bu, au []map[string]json.RawMessage
	if json.Unmarshal(before["users"], &bu) != nil || json.Unmarshal(after["users"], &au) != nil || len(bu) != 1 || len(au) != 1 {
		return p, quota.ErrConflict
	}
	var binding config.SQLiteBinding
	if json.Unmarshal(au[0]["sqlite_meter"], &binding) != nil || binding.Directory != p.Directory || binding.Store != p.Store {
		return p, quota.ErrConflict
	}
	if _, exists := bu[0]["sqlite_meter"]; exists {
		return p, quota.ErrConflict
	}
	delete(au[0], "sqlite_meter")
	x, _ := json.Marshal(bu)
	y, _ := json.Marshal(au)
	if string(x) != string(y) {
		return p, quota.ErrConflict
	}
	delete(before, "users")
	delete(after, "users")
	x, _ = json.Marshal(before)
	y, _ = json.Marshal(after)
	if string(x) != string(y) {
		return p, quota.ErrConflict
	}
	return p, nil
}
func ApplyUnlimitedMigration(dir string) error {
	return withPublicationLock(func() error {
		p, e := loadUnlimitedPlan(dir)
		if e != nil {
			return e
		}
		raw, e := os.ReadFile(p.ConfigPath)
		if e != nil {
			return e
		}
		before, after := digest(raw) == digest(p.Before), digest(raw) == digest(p.After)
		if !before && !after {
			return errors.New("config differs from recorded before/after; held")
		}
		c, e := config.LoadFrom(p.ConfigPath)
		if e != nil {
			return e
		}
		u, e := unlimitedUser(c)
		if e != nil {
			return e
		}
		if c.ServerID != p.Server || u.AccountID != p.Account || u.PeriodID != p.Period || u.Username != p.User {
			return quota.ErrConflict
		}
		anchor := p.ConfigPath + ".unlimited-migration.anchor"
		planBytes, e := os.ReadFile(filepath.Join(dir, "plan.json"))
		if e != nil {
			return e
		}
		identity := digest(planBytes)
		a, ae := os.ReadFile(anchor)
		if ae == nil {
			if string(a) != identity {
				return quota.ErrConflict
			}
		} else if !os.IsNotExist(ae) {
			return ae
		} else {
			if !before {
				return quota.ErrConflict
			}
			if _, err := os.Lstat(p.Directory); !os.IsNotExist(err) {
				return quota.ErrConflict
			}
		}
		receipt := filepath.Join(dir, "complete.json")
		if b, e := os.ReadFile(receipt); e == nil {
			if string(b) != p.Operation || !after {
				return quota.ErrConflict
			}
			s, e := quota.Open(context.Background(), p.Directory, p.Server, p.Store)
			if e != nil {
				return e
			}
			defer s.Close()
			m, e := s.Metadata(context.Background(), quota.Key{Account: p.Account, Period: p.Period})
			if e != nil || m.LimitKind != "unlimited" || m.Confirmed < p.Opening {
				return quota.ErrConflict
			}
			return nil
		} else if !os.IsNotExist(e) {
			return e
		}
		intent := filepath.Join(dir, "apply.pending")
		_, intentErr := os.Stat(intent)
		if os.IsNotExist(intentErr) {
			if !before {
				return quota.ErrConflict
			}
			snapshot, e := usage.RecordedSnapshot(p.User)
			if e != nil {
				return e
			}
			if snapshot.Fingerprint != p.UsageHash || snapshot.Total != p.Watermark {
				return errors.New("usage changed since plan; prepare a new plan in a NEW directory")
			}
			if e = safefile.Write(intent, []byte(p.Operation), 0600); e != nil {
				return e
			}
		} else if intentErr != nil {
			return intentErr
		} else {
			b, e := os.ReadFile(intent)
			if e != nil || string(b) != p.Operation {
				return quota.ErrConflict
			}
		}
		if os.IsNotExist(ae) {
			if e = safefile.Write(anchor, []byte(identity), 0600); e != nil {
				return e
			}
		}
		if e = unlimitedStep("intent"); e != nil {
			return e
		}
		ctx := context.Background()
		var s *quota.Store
		if _, e = os.Lstat(p.Directory); os.IsNotExist(e) {
			s, e = quota.Create(ctx, p.Directory, p.Server, p.Store)
		} else if e == nil {
			if _, pendingErr := os.Lstat(filepath.Join(p.Directory, "bootstrap.pending")); pendingErr == nil {
				s, e = quota.ResumeBootstrap(ctx, p.Directory, p.Server, p.Store)
			} else {
				s, e = quota.Open(ctx, p.Directory, p.Server, p.Store)
			}
		}
		if e != nil {
			return e
		}
		defer s.Close()
		if e = s.ImportUnlimited(ctx, quota.Key{Account: p.Account, Period: p.Period}, p.Operation, digest(p.Before)+":"+p.UsageHash, p.Opening, p.Watermark); e != nil {
			return e
		}
		if e = unlimitedStep("imported"); e != nil {
			return e
		}
		if _, e = os.Stat(filepath.Join(p.Directory, "bootstrap.pending")); e == nil {
			if e = s.FinishBootstrap(p.Directory); e != nil {
				return e
			}
		}
		if e = unlimitedStep("bootstrapped"); e != nil {
			return e
		}
		if before {
			if e = safefile.Write(p.ConfigPath, p.After, 0600); e != nil {
				return e
			}
		}
		if e = unlimitedStep("selected"); e != nil {
			return e
		}
		if e = safefile.Write(receipt, []byte(p.Operation), 0600); e != nil {
			return e
		}
		return unlimitedStep("receipt")
	})
}
func ObserveUnlimitedAccounts(c *config.Config) error {
	var failures []error
	for _, u := range c.Users {
		if u.SQLiteMeter == nil || u.QuotaGB != 0 {
			continue
		}
		if err := observeUnlimited(c.ServerID, u); err != nil {
			failures = append(failures, fmt.Errorf("account %s: %w", u.Username, err))
		}
	}
	return errors.Join(failures...)
}

func ownedMigrationPath(i os.FileInfo) bool {
	st, ok := i.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid()) && (i.IsDir() || st.Nlink == 1)
}

func observeUnlimited(server string, u config.UserConfig) error {
	snap, err := usage.RecordedSnapshot(u.Username)
	b := u.SQLiteMeter
	if err != nil && !errors.Is(err, usage.ErrNoAccountEvidence) {
		return err
	}
	ctx := context.Background()
	s, e := quota.Open(ctx, b.Directory, server, b.Store)
	if e != nil {
		return e
	}
	defer s.Close()
	k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
	if err != nil {
		m, e := s.Metadata(ctx, k)
		if e != nil {
			return e
		}
		if m.Confirmed != 0 || u.UsageBase != 0 {
			return err
		}
		return nil
	}
	return s.ObserveUnlimited(ctx, k, snap.Total, time.Now().Unix())
}
