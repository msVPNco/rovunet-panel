package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"
)

func exactInt64(raw json.RawMessage) (int64, error) {
	text := string(raw)
	if len(text) == 0 || len(text) > 128 || text[0] == '"' || !json.Valid(raw) {
		return 0, errors.New("invalid integer")
	}
	// Bound scientific notation before arbitrary precision allocation.
	if n := strings.IndexAny(text, "eE"); n >= 0 {
		exp, e := strconv.Atoi(text[n+1:])
		if e != nil || exp > 64 || exp < -128 {
			return 0, errors.New("integer exponent out of range")
		}
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok || !r.IsInt() || !r.Num().IsInt64() {
		return 0, errors.New("integer out of int64 range")
	}
	return r.Num().Int64(), nil
}

// ParsePrivateStatusV2 is the exact-number reference parser for adapters/tests.
// JSON numbers remain numbers; no value is carried through float64.
func ParsePrivateStatusV2(raw []byte) (StatusV2, error) {
	var v StatusV2
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return v, e
	}
	required := []string{"version", "server_id", "account_id", "period_id", "revision", "account_status", "limit_kind", "limit_bytes", "expiry_kind", "expires_at", "confirmed_bytes", "pending_bytes", "coverage", "measurement", "sampled_at", "generated_at"}
	if len(fields) != len(required) {
		return v, errors.New("status field count")
	}
	for _, k := range required {
		if _, ok := fields[k]; !ok {
			return v, errors.New("missing status field")
		}
	}
	for _, k := range []string{"version", "revision", "limit_bytes", "confirmed_bytes", "pending_bytes"} {
		b := fields[k]
		if bytes.Equal(b, []byte("null")) {
			if k == "version" || k == "revision" {
				return v, errors.New("required integer")
			}
			continue
		}
		n, e := exactInt64(b)
		if e != nil || n < 0 || (k == "revision" && n < 1) {
			return v, errors.New("invalid status integer")
		}
		fields[k] = json.RawMessage(strconv.FormatInt(n, 10))
	}
	canonical, e := json.Marshal(fields)
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal(canonical, &v); e != nil {
		return v, e
	}
	if v.Version != 2 || !validPublicationID(v.Server, v.Account, v.Period) {
		return v, errors.New("invalid binding or version")
	}
	if v.LimitKind == "finite" {
		if v.Limit == nil || *v.Limit <= 0 {
			return v, errors.New("finite limit")
		}
	} else if (v.LimitKind != "unknown" && v.LimitKind != "unlimited") || v.Limit != nil {
		return v, errors.New("limit kind")
	}
	if v.ExpiryKind == "at" {
		if v.Expires == nil {
			return v, errors.New("expiry missing")
		}
		if _, e = time.Parse(time.RFC3339Nano, *v.Expires); e != nil {
			return v, e
		}
	} else if (v.ExpiryKind != "none" && v.ExpiryKind != "unknown") || v.Expires != nil {
		return v, errors.New("expiry kind")
	}
	allowed := func(s string, values ...string) bool {
		for _, v := range values {
			if s == v {
				return true
			}
		}
		return false
	}
	if !allowed(v.Status, "active", "expired", "limit_reached", "hold", "archived") || !allowed(v.Coverage, "partial", "unavailable", "not_metered") || !allowed(v.Measurement, "accounted_lower_bound", "none", "unknown") {
		return v, errors.New("status enum")
	}
	if _, e = time.Parse(time.RFC3339Nano, v.Generated); e != nil {
		return v, e
	}
	if v.Sampled != nil {
		if _, e = time.Parse(time.RFC3339Nano, *v.Sampled); e != nil {
			return v, e
		}
	}
	return v, nil
}
