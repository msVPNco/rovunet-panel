package coreinstall

import (
	"context"
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"time"
)

// SamplePreviewQuota uses a pre-established baseline for a single bound core
// invocation/account/period. A restart requires a fresh reviewed binding.
func SamplePreviewQuota(ctx context.Context, s *quota.Store, configFile, server, core, user, account, period, epoch string, upBase, downBase int64) error {
	unlock, e := lockUsage()
	if e != nil {
		return e
	}
	defer unlock()
	readBinding := func() error {
		cfg, e := config.LoadFrom(configFile)
		if e != nil {
			return e
		}
		u := cfg.GetUser(user)
		if cfg.ServerID != server || u == nil || u.AccountID != account || u.PeriodID != period {
			return quota.ErrConflict
		}
		now, e := runEpoch(core)
		if e != nil || now != epoch {
			return quota.ErrConflict
		}
		return nil
	}
	if e = readBinding(); e != nil {
		return e
	}
	endpoint := coreEndpoint(core)
	if endpoint == "" {
		return errors.New("missing core endpoint")
	}
	raw, e := runStats(endpoint, false)
	if e != nil {
		return e
	}
	v, e := parseStatsValues(raw, true)
	if e != nil {
		return e
	}
	if e = readBinding(); e != nil {
		return e
	}
	stamp := time.Now().Unix()
	up, okU := v[user+">>>uplink"]
	down, okD := v[user+">>>downlink"]
	if !okU || !okD {
		return errors.New("incomplete per-direction sample")
	}
	op := fmt.Sprintf("%s/%s/%d/%d/%d", core, epoch, up, down, stamp)
	return s.ObservePair(ctx, quota.Key{Account: account, Period: period}, op, "xray-"+core, epoch, upBase, downBase, up, down, stamp)
}
