package handlers

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestStatusV2ExactInt64(t *testing.T) {
	cases := []struct {
		raw string
		ok  bool
	}{{"0", true}, {"1", true}, {"9007199254740993", true}, {"9223372036854775807", true}, {"9223372036854775808", false}, {"-1", false}, {"1.5", false}, {`"1"`, false}, {"1.0", true}, {"1e0", true}}
	original := `{"version":2,"server_id":"ffffffffffffffffffffffffffffffff","account_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","period_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","revision":1,"account_status":"active","limit_kind":"finite","limit_bytes":1,"expiry_kind":"none","expires_at":null,"confirmed_bytes":0,"pending_bytes":0,"coverage":"partial","measurement":"accounted_lower_bound","sampled_at":null,"generated_at":"2030-01-01T00:00:00Z"}`
	var output []map[string]any
	for _, field := range []string{"revision", "limit_bytes", "confirmed_bytes", "pending_bytes"} {
		for _, tc := range cases {
			var values map[string]json.RawMessage
			if e := json.Unmarshal([]byte(original), &values); e != nil {
				t.Fatal(e)
			}
			values[field] = json.RawMessage(tc.raw)
			b, e := json.Marshal(values)
			if e != nil {
				t.Fatal(e)
			}
			_, e = ParsePrivateStatusV2(b)
			wanted := tc.ok && !(tc.raw == "0" && (field == "limit_bytes" || field == "revision"))
			if (e == nil) != wanted {
				t.Fatalf("%s %s expected=%t error=%v", field, tc.raw, wanted, e)
			}
			output = append(output, map[string]any{"field": field, "raw": tc.raw, "accepted": e == nil, "expected": wanted})
		}
	}
	if path := os.Getenv("RV20_SCHEMA_RESULTS"); path != "" {
		b, e := json.MarshalIndent(output, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := exactInt64(json.RawMessage(strings.Repeat("9", 129))); e == nil {
		t.Fatal("unbounded integer accepted")
	}
}
