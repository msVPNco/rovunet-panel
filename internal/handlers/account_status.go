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
//	  "sampled_at": "2026-11-08T10:00:00Z", // last read of the Xray counters; null without a limit
//	  "generated_at": "2026-11-08T10:00:03Z",
//	  "revision": 7                     // +1 whenever anything but the times changes
//	}
//
// used_bytes counts both directions. metering_status is "partial" when some
// core did not answer the last read (the count may be low), "unavailable"
// when the counters were never read, and "not_metered" for a user without a
// traffic limit, whose traffic is not counted on every protocol.

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
	collection usage.Collection
	collected  bool
}

func newStatusInput(cfg *config.Config, now time.Time) statusInput {
	c, ok := usage.LastCollected()
	return statusInput{serverID: cfg.ServerID, now: now, totals: usage.Totals(), collection: c, collected: ok}
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
	case overQuota(u, in.totals):
		st.Status = statusLimitReached
	}
	if t, err := time.Parse(time.RFC3339, u.ExpiresAt); err == nil && u.ExpiresAt != "" {
		s := t.UTC().Format(time.RFC3339)
		st.ExpiresAt = &s
	}
	if u.QuotaGB > 0 {
		limit, used := u.QuotaBytes(), UsedBytes(u, in.totals)
		st.LimitBytes, st.UsedBytes = &limit, &used
		switch {
		case !in.collected:
			st.Metering = meteringUnavailable
		case in.collection.Complete:
			st.Metering = meteringComplete
		default:
			st.Metering = meteringPartial
		}
		if in.collected {
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
