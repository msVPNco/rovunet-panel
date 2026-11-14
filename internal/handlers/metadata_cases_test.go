package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
)

func i64(n int64) *int64 { return &n }
func TestTypedHeadersOverTLS(t *testing.T) {
	body := []byte(`{"version":1,"name":"fixture","items":[]}`)
	for _, c := range []struct {
		name string
		s    SubscriptionSnapshot
		want string
	}{
		{"volume", SubscriptionSnapshot{LimitKind: "finite", Limit: i64(1000), Upload: i64(10), Download: i64(20)}, "upload=10; download=20; total=1000"},
		{"time", SubscriptionSnapshot{ExpiryKind: "at", Expiry: i64(1800000000)}, "expire=1800000000"},
		{"combined", SubscriptionSnapshot{LimitKind: "finite", Limit: i64(1000), ExpiryKind: "at", Expiry: i64(1800000000)}, "total=1000; expire=1800000000"},
		{"unlimited", SubscriptionSnapshot{LimitKind: "unlimited"}, "total=0"},
		{"partial", SubscriptionSnapshot{Upload: i64(10)}, "upload=10"},
		{"bad", SubscriptionSnapshot{LimitKind: "finite", Limit: i64(-1), Upload: i64(-1), ExpiryKind: "at", Expiry: i64(1800000000000)}, ""},
		{"missing", SubscriptionSnapshot{}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.s.Body = body
			c.s.Title = "آزمایش"
			srv := httptest.NewTLSServer(SubscriptionMetadataHandler(func(context.Context, string, string) (SubscriptionSnapshot, error) { return c.s, nil }))
			defer srv.Close()
			for _, format := range []string{"json", "windows.json"} {
				rsp, e := srv.Client().Get(srv.URL + "/v2/sub/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa." + format)
				if e != nil {
					t.Fatal(e)
				}
				b, e := io.ReadAll(rsp.Body)
				rsp.Body.Close()
				if e != nil || string(b) != string(body) || rsp.StatusCode != 200 || rsp.Header.Get("Subscription-Userinfo") != c.want {
					t.Fatal(e, rsp.StatusCode, rsp.Header)
				}
				h, _ := json.Marshal(rsp.Header)
				t.Logf("format=%s body_sha256=%x headers_sha256=%x userinfo=%s", format, sha256.Sum256(b), sha256.Sum256(h), fmt.Sprint(c.want))
			}
		})
	}
}
