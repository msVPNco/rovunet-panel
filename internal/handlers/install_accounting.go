package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

// Reuse the persisted period as the store identity. A crash between publishing
// the private database and publishing its binding reopens the original period.
func installAccounting(cfg *config.Config, fresh map[string]bool) error {
	path := cfg.Path()
	if err := cfg.ValidateSubscriptionTokens(); err != nil {
		return err
	}
	if cfg.RecoverySealed || cfg.HasCarryHold("") {
		return config.ErrCoordinatedRecovery
	}
	if cfg.SubscriptionV2 || cfg.V2Authority != nil {
		if err := ValidateV2Authority(cfg); err != nil {
			return err
		}
	}
	if mintIDs(cfg) {
		if err := cfg.Save(); err != nil {
			return err
		}
	}
	err := withPublicationLock(func() error {
		if err := cfg.RefreshIfChanged(); err != nil {
			return err
		}
		return cfg.WithCurrentFile(func() error {
			if cfg.RecoverySealed || config.AccountOnHold(path, "") {
				return config.ErrCoordinatedRecovery
			}
			var top map[string]json.RawMessage
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(raw, &top); err != nil {
				return err
			}
			if len(cfg.Users) == 0 {
				return nil
			}
			var rows []map[string]json.RawMessage
			if err = json.Unmarshal(top["users"], &rows); err != nil || len(rows) != len(cfg.Users) {
				return errors.New("account list changed during installation")
			}
			changed := false
			for i, u := range cfg.Users {
				if u.SQLiteMeter != nil {
					if _, _, err := boundAmount(cfg.ServerID, u); err != nil {
						return err
					}
					continue
				}
				if u.QuotaGB != 0 {
					continue
				}
				if config.AccountOnHold(path, u.Username) {
					return config.ErrCoordinatedRecovery
				}
				if !validPublicationID(cfg.ServerID, u.AccountID, u.PeriodID, u.SubToken) {
					return errors.New("persist subscription identities before accounting setup")
				}
				snap, err := usage.RecordedSnapshot(u.Username)
				if errors.Is(err, usage.ErrNoAccountEvidence) {
					isNew, err := recordedNewPeriod(path, u)
					if err != nil {
						return err
					}
					if !fresh[u.AccountID] && !isNew {
						continue
					} // retain this account's existing meter until it has evidence
					if u.UsageBase != 0 {
						return errors.New("new account has a nonzero baseline without recorded usage")
					}
				} else if err != nil {
					return err
				}
				if snap.Total < u.UsageBase {
					return fmt.Errorf("recorded usage below period baseline for %s", u.Username)
				}
				binding, err := installPeriod(path, cfg.ServerID, u, snap.Total)
				if err != nil {
					return err
				}
				rows[i]["sqlite_meter"], err = json.Marshal(binding)
				if err != nil {
					return err
				}
				changed = true
			}
			if !changed {
				return nil
			}
			top["users"], err = json.Marshal(rows)
			if err != nil {
				return err
			}
			raw, err = json.MarshalIndent(top, "", "  ")
			if err != nil {
				return err
			}
			return safefile.Write(path, append(raw, '\n'), 0600)
		})
	})
	if err != nil {
		return err
	}
	return cfg.RefreshIfChanged()
}

func installPeriod(path, server string, u config.UserConfig, total int64) (config.SQLiteBinding, error) {
	ctx := context.Background()
	root := path + ".accounting"
	if path == config.DefaultConfigFile {
		root = filepath.Join("/var/lib/rovunet-quota", "accounts-"+digest([]byte(server))[:32])
	}
	dir := filepath.Join(root, u.PeriodID)
	b := config.SQLiteBinding{Directory: dir, Store: u.PeriodID}
	if err := os.MkdirAll(root, 0700); err != nil {
		return b, err
	}
	k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
	if _, err := os.Lstat(dir); err == nil {
		s, err := quota.Open(ctx, dir, server, b.Store)
		if err != nil {
			return b, err
		}
		defer s.Close()
		m, err := s.Metadata(ctx, k)
		if err != nil {
			return b, err
		}
		if m.LimitKind != "unlimited" {
			return b, quota.ErrConflict
		}
		if err = s.ObserveUnlimited(ctx, k, total, 0); err != nil {
			return b, err
		}
		m, err = s.Metadata(ctx, k)
		if err != nil || m.Confirmed != total-u.UsageBase {
			return b, quota.ErrConflict
		}
		return b, nil
	} else if !os.IsNotExist(err) {
		return b, err
	}
	tmp, err := os.MkdirTemp(root, ".prepare-")
	if err != nil {
		return b, err
	}
	defer os.RemoveAll(tmp)
	stage := filepath.Join(tmp, "period")
	s, err := quota.Create(ctx, stage, server, b.Store)
	if err != nil {
		return b, err
	}
	err = s.ImportUnlimited(ctx, k, u.PeriodID, "installed-period:"+u.AccountID, total-u.UsageBase, total)
	if err == nil {
		err = s.FinishBootstrap(stage)
	}
	closeErr := s.Close()
	if err != nil {
		return b, err
	}
	if closeErr != nil {
		return b, closeErr
	}
	if err = os.Rename(stage, dir); err != nil {
		return b, err
	}
	if err = safefile.SyncDir(root); err != nil {
		return b, err
	}
	return b, safefile.SyncDir(filepath.Dir(root))
}

// A new account may be saved while DNS or certificate setup is pending.
// Persist its genesis before adding it so a later completion can recognize a
// genuine zero-usage period without treating missing legacy samples as zero.
func rememberNewPeriod(cfg *config.Config, u config.UserConfig) error {
	if !validPublicationID(u.AccountID, u.PeriodID) {
		return quota.ErrConflict
	}
	dir := cfg.Path() + ".new-periods"
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	if e := safefile.Write(filepath.Join(dir, u.PeriodID), []byte(u.AccountID), 0600); e != nil {
		return e
	}
	return safefile.SyncDir(filepath.Dir(dir))
}
func recordedNewPeriod(path string, u config.UserConfig) (bool, error) {
	b, e := os.ReadFile(filepath.Join(path+".new-periods", u.PeriodID))
	if os.IsNotExist(e) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if string(b) != u.AccountID {
		return false, quota.ErrConflict
	}
	return true, nil
}
