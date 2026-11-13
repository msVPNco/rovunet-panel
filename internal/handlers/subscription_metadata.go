package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type SubscriptionSnapshot struct {
	Revision                        int64
	Generated                       string
	Body                            []byte
	Title                           string
	LimitKind, ExpiryKind           string
	Limit, Expiry, Upload, Download *int64
}

func (s SubscriptionSnapshot) userinfo() string {
	fields := []string{}
	for _, pair := range []struct {
		name string
		n    *int64
	}{{"upload", s.Upload}, {"download", s.Download}} {
		if pair.n != nil && *pair.n >= 0 {
			fields = append(fields, fmt.Sprintf("%s=%d", pair.name, *pair.n))
		}
	}
	switch s.LimitKind {
	case "finite":
		if s.Limit != nil && *s.Limit > 0 {
			fields = append(fields, "total="+strconv.FormatInt(*s.Limit, 10))
		}
	case "unlimited":
		if s.Limit == nil {
			fields = append(fields, "total=0")
		}
	}
	if s.ExpiryKind == "at" && s.Expiry != nil && *s.Expiry > 0 && *s.Expiry <= 253402300799 {
		fields = append(fields, "expire="+strconv.FormatInt(*s.Expiry, 10))
	}
	return strings.Join(fields, "; ")
}

// Resolve must return an authorized immutable snapshot; empty bodies map to 404.
func SubscriptionMetadataHandler(resolve func(context.Context, string, string) (SubscriptionSnapshot, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		if r.URL.RawPath != "" || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		stem, ok := strings.CutPrefix(r.URL.Path, "/v2/sub/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		token, format, ok := strings.Cut(stem, ".")
		if !ok || !tokenOK(token) || (format != "json" && format != "windows.json" && format != "status.json" && format != "windows.status.json") {
			http.NotFound(w, r)
			return
		}
		s, e := resolve(r.Context(), token, format)
		if e != nil {
			if errors.Is(e, errNoFeed) {
				http.NotFound(w, r)
			} else {
				http.Error(w, "snapshot unavailable", 503)
			}
			return
		}
		if len(s.Body) == 0 {
			http.NotFound(w, r)
			return
		}
		if s.Revision > 0 {
			w.Header().Set("Rovunet-Revision", strconv.FormatInt(s.Revision, 10))
		}
		if s.Generated != "" {
			w.Header().Set("Rovunet-Generated-At", s.Generated)
		}
		if h := s.userinfo(); h != "" {
			w.Header().Set("Subscription-Userinfo", h)
		}
		if utf8.ValidString(s.Title) && len(s.Title) > 0 && len(s.Title) <= 256 && !strings.ContainsAny(s.Title, "\r\n") {
			w.Header().Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(s.Title)))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_, _ = w.Write(s.Body)
		}
	})
}

var errNoFeed = errors.New("subscription absent")

func ResolveMetadataFromConfig(path string, store *quota.Store, expectedServer string) func(context.Context, string, string) (SubscriptionSnapshot, error) {
	return func(ctx context.Context, token, format string) (SubscriptionSnapshot, error) {
		cfg, e := config.LoadFrom(path)
		if e != nil {
			return SubscriptionSnapshot{}, e
		}
		if e = cfg.ValidateSubscriptionTokens(); e != nil {
			return SubscriptionSnapshot{}, e
		}
		for _, u := range cfg.Users {
			if u.SubToken != token {
				continue
			}
			if e = uniqueSubscriptionOwner(cfg, token, u.AccountID); e != nil {
				return SubscriptionSnapshot{}, e
			}
			now := time.Now()
			var measurement quota.Metadata
			if u.SQLiteMeter != nil && (store == nil || !store.MatchesBinding(u.SQLiteMeter.Directory, cfg.ServerID, u.SQLiteMeter.Store)) {
				return SubscriptionSnapshot{}, quota.ErrConflict
			}
			if u.Expired(now) {
				return SubscriptionSnapshot{}, errNoFeed
			}
			if store != nil && u.QuotaGB > 0 {
				if expectedServer != cfg.ServerID {
					return SubscriptionSnapshot{}, quota.ErrConflict
				}
				measurement, e = store.Metadata(ctx, quota.Key{Account: u.AccountID, Period: u.PeriodID})
				if e != nil {
					return SubscriptionSnapshot{}, e
				}
				if measurement.Limit != u.QuotaBytes() {
					return SubscriptionSnapshot{}, quota.ErrConflict
				}
				if measurement.Confirmed >= measurement.Limit {
					return SubscriptionSnapshot{}, errNoFeed
				}
			} else {
				st := buildStatus(newStatusInput(cfg, now), u, "")
				if st.Status != statusActive {
					return SubscriptionSnapshot{}, errNoFeed
				}
			}
			client := ""
			if format == "windows.json" {
				client = "windows"
			}
			body, e := buildFeedForClient(cfg, u, client)
			if e != nil {
				return SubscriptionSnapshot{}, e
			}
			v := SubscriptionSnapshot{Body: body, Title: u.Username, LimitKind: "unknown", ExpiryKind: "unknown"}
			v.Upload = measurement.Upload
			v.Download = measurement.Download
			if u.QuotaGB == 0 {
				v.LimitKind = "unlimited"
			} else if u.QuotaGB > 0 && u.QuotaGB <= math.MaxInt64/(1<<30) {
				n := u.QuotaBytes()
				v.LimitKind = "finite"
				v.Limit = &n
			}
			if u.ExpiresAt == "" {
				v.ExpiryKind = "none"
			} else if t, e := time.Parse(time.RFC3339, u.ExpiresAt); e == nil {
				n := t.Unix()
				v.ExpiryKind = "at"
				v.Expiry = &n
			}
			original, e := cfg.WithSnapshotDigest()
			if e != nil {
				return SubscriptionSnapshot{}, e
			}
			fresh, e := config.LoadFrom(path)
			if e != nil {
				return SubscriptionSnapshot{}, e
			}
			current, e := fresh.WithSnapshotDigest()
			if e != nil || original != current {
				return SubscriptionSnapshot{}, errNoFeed
			}
			a := fresh.GetUser(u.Username)
			if a == nil || fresh.ServerID != cfg.ServerID || !reflect.DeepEqual(*a, u) {
				return SubscriptionSnapshot{}, errNoFeed
			}
			return v, nil
		}
		return SubscriptionSnapshot{}, errNoFeed
	}
}
