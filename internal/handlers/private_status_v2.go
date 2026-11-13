package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
)

type StatusV2 struct {
	Version     int     `json:"version"`
	Server      string  `json:"server_id"`
	Account     string  `json:"account_id"`
	Period      string  `json:"period_id"`
	Revision    int64   `json:"revision"`
	Status      string  `json:"account_status"`
	LimitKind   string  `json:"limit_kind"`
	Limit       *int64  `json:"limit_bytes"`
	ExpiryKind  string  `json:"expiry_kind"`
	Expires     *string `json:"expires_at"`
	Confirmed   *int64  `json:"confirmed_bytes"`
	Pending     *int64  `json:"pending_bytes"`
	Coverage    string  `json:"coverage"`
	Measurement string  `json:"measurement"`
	Sampled     *string `json:"sampled_at"`
	Generated   string  `json:"generated_at"`
}
type statusRecord struct {
	Server, Account, Period, Semantic string
	Revision                          int64
	Confirmed                         *int64
}

// Revision is private durable state, scoped to installation/account/period;
// token/format and timestamps do not create revisions or new identities.
func commitStatusRevision(cfg *config.Config, v *StatusV2) error {
	lock, e := revisionLock(cfg, true)
	if e != nil {
		return e
	}
	defer lock.Close()
	registry, before, e := readRevision(cfg)
	if e != nil {
		return e
	}
	records := registry.Records
	key := digest([]byte(v.Server + "/" + v.Account + "/" + v.Period))
	old, exists := records[key]
	if exists && (old.Server != v.Server || old.Account != v.Account || old.Period != v.Period || old.Revision <= 0) {
		return errors.New("invalid private revision state")
	}
	if old.Confirmed != nil && v.Confirmed != nil && *v.Confirmed < *old.Confirmed {
		return errors.New("confirmed usage regressed; account on hold")
	}
	semantic := *v
	semantic.Generated = ""
	semantic.Sampled = nil
	semantic.Revision = 0
	raw, e := json.Marshal(semantic)
	if e != nil {
		return e
	}
	hash := digest(raw)
	next := old.Revision
	if old.Semantic != hash {
		if next == math.MaxInt64 {
			return errors.New("revision overflow")
		}
		next++
	}
	rec := statusRecord{v.Server, v.Account, v.Period, hash, next, v.Confirmed}
	if rec.Confirmed == nil {
		rec.Confirmed = old.Confirmed
	}
	records[key] = rec
	if e = persistRevision(cfg, before, registry); e != nil {
		return e
	}
	v.Revision = next
	return nil
}

func statusV2Snapshot(ctx context.Context, cfg *config.Config, u config.UserConfig, store *quota.Store, expectedServer string, reason string) (StatusV2, quota.Metadata, error) {
	now := time.Now().UTC()
	v := StatusV2{Version: 2, Server: cfg.ServerID, Account: u.AccountID, Period: u.PeriodID, Status: "active", LimitKind: "unknown", ExpiryKind: "unknown", Coverage: "unavailable", Measurement: "unknown", Generated: now.Format(time.RFC3339Nano)}
	var m quota.Metadata
	if !validPublicationID(v.Server, v.Account, v.Period) {
		return v, m, quota.ErrConflict
	}
	if u.QuotaGB == 0 {
		v.LimitKind = "unlimited"
		v.Coverage = "not_metered"
		v.Measurement = "none"
	} else if u.QuotaGB > 0 && int64(u.QuotaGB) <= math.MaxInt64/(1<<30) {
		n := u.QuotaBytes()
		v.LimitKind = "finite"
		v.Limit = &n
	} else {
		v.Status = "hold"
	}
	if u.ExpiresAt == "" {
		v.ExpiryKind = "none"
	} else if t, e := time.Parse(time.RFC3339, u.ExpiresAt); e == nil {
		v.ExpiryKind = "at"
		s := t.UTC().Format(time.RFC3339Nano)
		v.Expires = &s
		if !now.Before(t) {
			v.Status = "expired"
		}
	} else {
		v.Status = "hold"
	}
	if u.QuotaGB > 0 {
		if store != nil {
			if expectedServer != cfg.ServerID || (u.SQLiteMeter != nil && !store.MatchesBinding(u.SQLiteMeter.Directory, cfg.ServerID, u.SQLiteMeter.Store)) {
				v.Status = "hold"
			} else {
				var e error
				m, e = store.Metadata(ctx, quota.Key{Account: u.AccountID, Period: u.PeriodID})
				if e != nil || v.Limit == nil || m.Limit != *v.Limit {
					v.Status = "hold"
				} else {
					v.Confirmed = &m.Confirmed
					v.Pending = &m.Pending
					v.Measurement = "accounted_lower_bound"
					v.Coverage = "partial"
					if m.SampledAt != nil {
						t := time.Unix(*m.SampledAt, 0).UTC().Format(time.RFC3339)
						v.Sampled = &t
					}
					if m.State != "active" {
						v.Status = "hold"
					} else if m.Confirmed >= m.Limit && v.Status == "active" {
						v.Status = "limit_reached"
					}
				}
			}
		} else {
			v.Status = "hold"
		}
	}
	if config.AccountOnHold(cfg.Path(), u.Username) || cfg.RecoverySealed {
		v.Status = "hold"
	}
	if reason != "" {
		v.Status = "archived"
		if reason == "expired" {
			v.Status = "expired"
		} else if reason == "traffic limit" {
			v.Status = "limit_reached"
		}
	}
	if e := commitStatusRevision(cfg, &v); e != nil {
		return v, m, e
	}
	return v, m, nil
}

// This resolver is explicit about schema2 on private status, while feeds remain
// strict schema1. It can return a hold status for a broken configured ledger;
// a feed never grants new access using that unknown state.
func ResolveV2FromConfig(path string, override *quota.Store, server string) func(context.Context, string, string) (SubscriptionSnapshot, error) {
	return resolveV2(path, override, server, false)
}
func ResolveInstalledV2(path string) func(context.Context, string, string) (SubscriptionSnapshot, error) {
	return resolveV2(path, nil, "", true)
}
func resolveV2(path string, override *quota.Store, server string, installed bool) func(context.Context, string, string) (SubscriptionSnapshot, error) {
	return func(ctx context.Context, token, format string) (SubscriptionSnapshot, error) {
		cfg, e := config.LoadFrom(path)
		if e != nil {
			return SubscriptionSnapshot{}, e
		}
		if installed && !cfg.SubscriptionV2 {
			return SubscriptionSnapshot{}, errNoFeed
		}
		if e = cfg.ValidateSubscriptionTokens(); e != nil {
			return SubscriptionSnapshot{}, e
		}
		var u *config.UserConfig
		reason := ""
		for _, candidate := range cfg.Users {
			if candidate.SubToken == token {
				v := candidate
				if u != nil {
					return SubscriptionSnapshot{}, quota.ErrConflict
				}
				u = &v
			}
		}
		if u == nil {
			for _, a := range cfg.Archived {
				if a.SubToken == token {
					v := a.UserConfig
					if u != nil {
						return SubscriptionSnapshot{}, quota.ErrConflict
					}
					u = &v
					reason = a.Reason
				}
			}
		}
		if u == nil {
			return SubscriptionSnapshot{}, errNoFeed
		}
		var store = override
		expected := server
		if u.SQLiteMeter != nil {
			b := u.SQLiteMeter
			store, e = quota.Open(ctx, b.Directory, cfg.ServerID, b.Store)
			expected = cfg.ServerID
			if e != nil {
				store = nil
			} else {
				defer store.Close()
			}
		}
		v, m, e := statusV2Snapshot(ctx, cfg, *u, store, expected, reason)
		if e != nil {
			return SubscriptionSnapshot{}, e
		}
		snap := SubscriptionSnapshot{Title: u.Username, LimitKind: v.LimitKind, Limit: v.Limit, ExpiryKind: v.ExpiryKind, Upload: m.Upload, Download: m.Download}
		if v.Expires != nil {
			t, _ := time.Parse(time.RFC3339Nano, *v.Expires)
			n := t.Unix()
			snap.Expiry = &n
		}
		snap.Revision = v.Revision
		snap.Generated = v.Generated
		if strings.HasSuffix(format, "status.json") {
			snap.Body, e = json.Marshal(v)
		} else {
			if v.Status != "active" {
				if v.Status == "hold" {
					return snap, errors.New("account on hold")
				}
				return snap, errNoFeed
			}
			client := ""
			if format == "windows.json" {
				client = "windows"
			}
			// Legacy finite users on v2 have unknown usage unless an explicit
			// authoritative ledger was supplied: do not invent zero.
			snap.Body, e = buildFeedForClient(cfg, *u, client)
		}
		if e != nil {
			return snap, e
		}
		if e = cfg.VerifyCurrentSnapshot(); e != nil {
			return SubscriptionSnapshot{}, fmt.Errorf("configuration snapshot changed: %w", e)
		}
		return snap, nil
	}
}
