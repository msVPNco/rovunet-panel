package handlers

import (
	"context"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"math"
	"path/filepath"
	"time"
)

func rotationEligible(cfg *config.Config, u config.UserConfig) bool {
	if cfg.RecoverySealed || cfg.HasCarryHold(u.Username) {
		return false
	}
	if u.ExpiresAt != "" {
		t, e := time.Parse(time.RFC3339, u.ExpiresAt)
		if e != nil || !time.Now().Before(t) {
			return false
		}
	}
	if u.SQLiteMeter != nil {
		a, lim, e := boundAmount(cfg.ServerID, u)
		return e == nil && a.Confirmed < lim
	}
	return buildStatus(newStatusInput(cfg, time.Now()), u, "").Status == statusActive
}

func boundAmount(server string, u config.UserConfig) (quota.Amount, int64, error) {
	b := u.SQLiteMeter
	if b == nil || !filepath.IsAbs(b.Directory) || b.Store == "" || u.QuotaGB <= 0 || int64(u.QuotaGB) > math.MaxInt64/(1<<30) {
		return quota.Amount{}, 0, quota.ErrConflict
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s, e := quota.Open(ctx, b.Directory, server, b.Store)
	if e != nil {
		return quota.Amount{}, 0, e
	}
	defer s.Close()
	k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
	lim, e := s.Limit(ctx, k)
	if e != nil || lim != u.QuotaBytes() {
		return quota.Amount{}, 0, quota.ErrConflict
	}
	a, e := s.Read(ctx, k)
	return a, lim, e
}
