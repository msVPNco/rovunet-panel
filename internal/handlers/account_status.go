package handlers

import (
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

// Each user also has a status file next to the subscription, at
// /sub/<token>.status.json: the same link with ".json" replaced by
// ".status.json". The subscription itself stays as it was, because the app
// rejects any key it does not know. The status file says what the account may
// still use, so the app can show it; it grants nothing; the server enforces
// the limits on its own. It is rewritten on every user change and on every
// limits check (every 5 minutes), and it stays after the user is stopped, so
// the app can say why the configs stopped working. A user removed for good
// has no file (404): the app cannot tell that apart from a server that does
// not have the file, so it must not read a 404 as "expired".
//
//	{
//	  "version": 1,
//	  "server_id": "<32 hex>",          // this panel; random, never changes
//	  "account_id": "<32 hex>",         // this account; kept by renew and passwd
//	  "period_id": "<32 hex>",          // new on every renew
//	  "account_status": "active",       // active | expired | limit_reached
//	  "expires_at": "2026-12-01T00:00:00Z", // or null: no expiry
//	  "limit_bytes": 10737418240,       // or null: no traffic limit (1 GB = 1024^3)
//	  "used_bytes": 1234,               // or null when there is no traffic limit
//	  "metering_status": "complete",    // complete | partial | unavailable | not_metered
//	  "sampled_at": "2026-11-08T10:00:00Z", // last complete read of the Xray counters, or null
//	  "generated_at": "2026-11-08T10:00:03Z",
//	  "revision": 7                     // +1 whenever anything but the times changes
//	}
//
// used_bytes counts both directions. metering_status is "partial" when some
// core did not answer the last read or a totals file could not be read (the
// count may be low), "unavailable" when the counters were never read, and
// "not_metered" for a user without a traffic limit, whose traffic is not
// counted on every protocol. sampled_at moves only on a read in which every
// core answered, so a failed read never makes an old count look fresh; it is
// null without a traffic limit or before the first complete read.
//
// revision belongs to the URL: it is continued from the file at the same
// path. A password change gives a new token, so the new URL starts again at
// 1 with the same account_id and period_id; an app that follows a new link
// must compare from scratch.

const statusSuffix = ".status.json"

// Values of account_status and metering_status.
const (
	statusActive       = "active"
	statusExpired      = "expired"
	statusLimitReached = "limit_reached"

	meteringComplete    = "complete"
	meteringPartial     = "partial"
	meteringUnavailable = "unavailable"
	meteringNone        = "not_metered"
)

type accountStatus struct {
	Version     int     `json:"version"`
	ServerID    string  `json:"server_id"`
	AccountID   string  `json:"account_id"`
	PeriodID    string  `json:"period_id"`
	Status      string  `json:"account_status"`
	ExpiresAt   *string `json:"expires_at"`
	LimitBytes  *int64  `json:"limit_bytes"`
	UsedBytes   *int64  `json:"used_bytes"`
	Metering    string  `json:"metering_status"`
	SampledAt   *string `json:"sampled_at"`
	GeneratedAt string  `json:"generated_at"`
	Revision    int64   `json:"revision"`
}

// statusInput is what every status file of one run shares.
type statusInput struct {
	serverID   string
	now        time.Time
	totals     map[string]int64
	totalsOK   bool // every totals file was read
	collection usage.Collection
	collected  bool // the cores' counters were read at least once
}

func newStatusInput(cfg *config.Config, now time.Time) statusInput {
	c, ok := usage.LastCollected()
	totals, err := usage.TotalsChecked()
	return statusInput{serverID: cfg.ServerID, now: now, totals: totals, totalsOK: err == nil, collection: c, collected: ok}
}

// buildStatus describes one account. reason is "" for a current user, or
// the reason a stopped user was stopped ("expired" or "traffic limit").
func buildStatus(in statusInput, u config.UserConfig, reason string) accountStatus {
	st := accountStatus{
		Version:     1,
		ServerID:    in.serverID,
		AccountID:   u.AccountID,
		PeriodID:    u.PeriodID,
		Status:      statusActive,
		GeneratedAt: in.now.UTC().Format(time.RFC3339),
	}
	switch {
	case reason == "expired":
		st.Status = statusExpired
	case reason != "":
		st.Status = statusLimitReached
	case u.Expired(in.now):
		st.Status = statusExpired // the next limits check stops the user
	case u.SQLiteMeter == nil && overQuota(u, in.totals):
		st.Status = statusLimitReached
	}
	if t, err := time.Parse(time.RFC3339, u.ExpiresAt); err == nil && u.ExpiresAt != "" {
		s := t.UTC().Format(time.RFC3339)
		st.ExpiresAt = &s
	}
	if u.SQLiteMeter != nil {
		limit := u.QuotaBytes()
		if u.QuotaGB > 0 {
			st.LimitBytes = &limit
		}
		a, lim, e := boundAmount(in.serverID, u)
		if e != nil {
			st.Metering = meteringUnavailable
			return st
		}
		// Released status1 readers require the original unlimited shape.
		// Keep the durable lower bound in SQLite/status2; do not expose it as
		// a partial metered status with a null limit in the legacy schema.
		if u.QuotaGB == 0 {
			st.Metering = meteringNone
			return st
		}
		st.UsedBytes = &a.Confirmed
		st.Metering = meteringPartial
		if u.QuotaGB > 0 && a.Confirmed >= lim && st.Status == statusActive {
			st.Status = statusLimitReached
		}
		return st
	}
	if u.QuotaGB > 0 {
		limit, used := u.QuotaBytes(), UsedBytes(u, in.totals)
		st.LimitBytes, st.UsedBytes = &limit, &used
		switch {
		case !in.collected:
			st.Metering = meteringUnavailable
		case in.collection.Complete && in.totalsOK && in.now.Sub(in.collection.Tried) <= 10*time.Minute:
			st.Metering = meteringComplete
		default:
			// A core did not answer the last read, or a totals file could
			// not be read: the count may be low.
			st.Metering = meteringPartial
		}
		if !in.collection.At.IsZero() {
			s := in.collection.At.UTC().Format(time.RFC3339)
			st.SampledAt = &s
		}
	} else {
		st.Metering = meteringNone
	}
	return st
}

// statusBody renders a status, continuing the revision of the file at path:
// the same revision if only the times changed, the next one otherwise.
func statusBody(st accountStatus, path string) ([]byte, error) {
	st.Revision = 1
	if old, err := os.ReadFile(path); err == nil {
		var prev accountStatus
		if json.Unmarshal(old, &prev) == nil && prev.Revision > 0 {
			// Released clients require a numeric lower bound on metered
			// accounts, even while counting is unavailable. Keep the last
			// known value instead of publishing a fabricated drop to zero.
			if prev.ServerID == st.ServerID && prev.AccountID == st.AccountID && prev.PeriodID == st.PeriodID && prev.UsedBytes != nil && st.UsedBytes != nil && *st.UsedBytes < *prev.UsedBytes {
				st.UsedBytes = prev.UsedBytes
				st.Metering = meteringPartial
				st.SampledAt = prev.SampledAt
				if st.Status == statusActive && st.LimitBytes != nil && *st.UsedBytes >= *st.LimitBytes {
					st.Status = statusLimitReached
				}
			}
			st.Revision = prev.Revision
			if !sameStatus(prev, st) {
				st.Revision++
			}
		}
	}
	return json.MarshalIndent(st, "", "  ")
}

// sameStatus compares two statuses without their times and revision.
func sameStatus(a, b accountStatus) bool {
	for _, s := range []*accountStatus{&a, &b} {
		s.SampledAt, s.GeneratedAt, s.Revision = nil, "", 0
	}
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}
