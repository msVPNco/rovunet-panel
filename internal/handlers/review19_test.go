package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
)

func bound19(t *testing.T) (*config.Config, *quota.Store, quota.Key) {
	c := fixture16(t)
	u := c.Users[0]
	u.QuotaGB = 1
	u.UsageBase = 99999
	dir := filepath.Join(t.TempDir(), "db")
	s, e := quota.Create(context.Background(), dir, c.ServerID, "s")
	ok16(t, e)
	t.Cleanup(func() { s.Close() })
	k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
	ok16(t, s.NewPeriod(context.Background(), k, 1<<30))
	ok16(t, s.FinishBootstrap(dir))
	u.SQLiteMeter = &config.SQLiteBinding{Directory: dir, Store: "s"}
	c.AddUser(u)
	secret := filepath.Join(t.TempDir(), "secret")
	ok16(t, os.WriteFile(secret, []byte(strings.Repeat("17", 32)), 0600))
	c.Tunnels = append(c.Tunnels, config.TunnelConfig{Tag: "vless", Enabled: true, Transport: config.TransportVLESS, Domain: "fixture.test", VLESS: &config.VLESSConfig{Port: 443, Path: "/ws", SecretFile: secret, TLS: true, CDN: true}})
	ok16(t, c.Save())
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	ok16(t, InitializeV2Authority(c))
	return c, s, k
}
func TestV2StatusLifecycle(t *testing.T) {
	c, s, k := bound19(t)
	ctx := context.Background()
	resolve := ResolveV2FromConfig(c.Path(), nil, "")
	read := func(format string) (StatusV2, SubscriptionSnapshot) {
		snap, e := resolve(ctx, c.Users[0].SubToken, format)
		ok16(t, e)
		var v StatusV2
		ok16(t, json.Unmarshal(snap.Body, &v))
		return v, snap
	}
	a, snap := read("status.json")
	b, _ := read("windows.status.json")
	if a.Revision != b.Revision || a.Server != b.Server || a.Version != 2 || *a.Confirmed != 0 || *a.Pending != 0 || a.Sampled != nil || a.Coverage != "partial" || a.Limit == nil || *a.Limit != 1<<30 {
		t.Fatal(a, b)
	}
	if snap.Upload != nil || snap.Download != nil || strings.Contains(snap.userinfo(), "upload=") {
		t.Fatal("invented directions")
	}
	feed, e := resolve(ctx, c.Users[0].SubToken, "json")
	ok16(t, e)
	var body map[string]any
	ok16(t, json.Unmarshal(feed.Body, &body))
	if len(body) != 3 || body["version"] != float64(1) {
		t.Fatal("broke feed v1")
	}
	ok16(t, s.ReserveNative(ctx, k, "T", "up", 100))
	p, _ := read("status.json")
	if *p.Pending != 100 || *p.Confirmed != 0 || p.Revision <= a.Revision || p.Status != "active" {
		t.Fatal(p)
	}
	ok16(t, s.Settle(ctx, k, "T", 13))
	settled, _ := read("status.json")
	if *settled.Confirmed != 13 || *settled.Pending != 0 || settled.Revision <= p.Revision {
		t.Fatal(settled)
	}
	// Rotate URL: identity/revision remain the same semantic snapshot.
	old := c.Users[0].SubToken
	ok16(t, RotateSubscriptionOperation(c, "alice", config.NewID()))
	after, _ := read("status.json")
	if after.Revision != settled.Revision || after.Account != settled.Account {
		t.Fatal("token created new identity")
	}
	if _, e = resolve(ctx, old, "status.json"); e == nil {
		t.Fatal("retired token authorized")
	}
	// Wrong store yields explicit hold and unknown amounts, never zero/fallback.
	u := c.Users[0]
	original := u.SQLiteMeter
	u.SQLiteMeter = &config.SQLiteBinding{Directory: original.Directory, Store: "wrong"}
	c.AddUser(u)
	ok16(t, c.Save())
	hold, _ := read("status.json")
	if hold.Status != "hold" || hold.Confirmed != nil || hold.Pending != nil {
		t.Fatal(hold)
	}
	if _, e = resolve(ctx, u.SubToken, "json"); e == nil {
		t.Fatal("hold feed authorized")
	}
	u.SQLiteMeter = original
	c.AddUser(u)
	ok16(t, c.Save())
	ok16(t, s.ReserveNative(ctx, k, "last", "down", (1<<30)-13))
	ok16(t, s.Settle(ctx, k, "last", (1<<30)-13))
	full, _ := read("status.json")
	if full.Status != "limit_reached" || *full.Confirmed != 1<<30 {
		t.Fatal(full)
	}
	// Missing durable revision registry must not silently restart at 1.
	registry := filepath.Join(c.Path()+".status-v2", "records.json")
	ok16(t, os.Rename(registry, registry+".retained"))
	if _, e = resolve(ctx, u.SubToken, "status.json"); e == nil {
		t.Fatal("missing revision authority reset")
	}
	ok16(t, os.Rename(registry+".retained", registry))
	st := buildStatus(newStatusInput(c, time.Now()), u, "")
	if st.UsedBytes == nil || *st.UsedBytes != 1<<30 {
		t.Fatal("UsageBase subtracted twice")
	}
}
func TestV2ContractFixtures(t *testing.T) {
	c := fixture16(t)
	ok16(t, InitializeV2Authority(c))
	ctx := context.Background()
	resolve := ResolveV2FromConfig(c.Path(), nil, "")
	u := c.Users[0]
	cases := []struct {
		name         string
		gb           int
		expiry       string
		kind, status string
	}{
		{"unlimited", 0, "", "unlimited", "active"}, {"time-only", 0, "2030-01-01T00:00:00Z", "unlimited", "active"}, {"volume-unknown", 1, "", "finite", "hold"}, {"mixed-unknown", 1, "2030-01-01T00:00:00Z", "finite", "hold"}, {"malformed-expiry", 0, "broken", "unlimited", "hold"}, {"expired", 0, "2000-01-01T00:00:00Z", "unlimited", "expired"}, {"invalid-limit", -1, "", "unknown", "hold"},
	}
	fixtures := map[string]any{}
	for _, tc := range cases {
		u.QuotaGB = tc.gb
		u.ExpiresAt = tc.expiry
		c.AddUser(u)
		ok16(t, c.Save())
		snap, e := resolve(ctx, u.SubToken, "status.json")
		ok16(t, e)
		var v StatusV2
		ok16(t, json.Unmarshal(snap.Body, &v))
		if v.Status != tc.status || v.LimitKind != tc.kind || v.Confirmed != nil || v.Pending != nil || v.Sampled != nil {
			t.Fatal(tc.name, v)
		}
		fixtures[tc.name] = map[string]any{"body": v, "subscription_userinfo": snap.userinfo()}
	}
	handler := SubscriptionMetadataHandler(resolve)
	for _, path := range []string{"/v2/sub/missing.status.json", "/v2/sub/" + u.SubToken + ".status.json?x=1", "/v2/sub/" + u.SubToken + ".other.json", "/v2/sub/%2e%2e/config.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
	good := httptest.NewRecorder()
	handler.ServeHTTP(good, httptest.NewRequest("GET", "/v2/sub/"+u.SubToken+".status.json", nil))
	if good.Code != 200 || good.Header().Get("Cache-Control") != "no-store" || good.Header().Get("Content-Type") != "application/json" {
		t.Fatal(good.Code, good.Header())
	}
	fixtures["negative"] = map[string]any{"missing_status_http": 404, "malformed_identity_accept": false, "conflicting_server_accept": false, "token_rewrites_source_item_pin": false}
	if dir := os.Getenv("RV19_FIXTURE_DIR"); dir != "" {
		b, e := json.MarshalIndent(fixtures, "", "  ")
		ok16(t, e)
		ok16(t, os.WriteFile(filepath.Join(dir, "status-v2-fixtures.json"), b, 0600))
	}
}
func TestCoherentForwardReconcile(t *testing.T) {
	c, s, k := bound19(t)
	ctx := context.Background()
	ok16(t, RotateSubscriptionOperation(c, "alice", config.NewID()))
	_, e0 := ResolveV2FromConfig(c.Path(), nil, "")(ctx, c.Users[0].SubToken, "status.json")
	ok16(t, e0)
	snapDir := filepath.Join(t.TempDir(), "snapshot")
	hash, e := CreateCoherentSnapshot(ctx, c, snapDir, config.NewID())
	ok16(t, e)
	if _, e = os.Stat(filepath.Join(snapDir, "status-revisions.anchor")); e != nil {
		t.Fatal("revision anchor missing", e)
	}
	if _, _, e = encodeBackup(c); e == nil {
		t.Fatal("legacy backup accepted")
	}
	ok16(t, s.ReserveNative(ctx, k, "after-snapshot", "up", 40))
	ok16(t, s.Settle(ctx, k, "after-snapshot", 40))
	ok16(t, ReconcileCoherentSnapshot(ctx, c, snapDir, hash))
	a, e := s.Read(ctx, k)
	ok16(t, e)
	if a.Confirmed != 40 {
		t.Fatal("rolled back usage", a)
	}
	// A later URL operation must invalidate the old config snapshot.
	ok16(t, RotateSubscriptionOperation(c, "alice", config.NewID()))
	if e = ReconcileCoherentSnapshot(ctx, c, snapDir, hash); e == nil {
		t.Fatal("old capability resurrected")
	}
	if e = ReconcileCoherentSnapshot(ctx, c, snapDir, "wrong"); e == nil {
		t.Fatal("untrusted manifest accepted")
	}
}
