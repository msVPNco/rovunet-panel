package coreplan

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestSupplementalStableAccountsAndTransport(t *testing.T) {
	for _, p := range []string{"anytls", "tuic", "mieru-tcp", "mieru-udp", "tlsvision", "xhttp-reality"} {
		t.Run(p, func(t *testing.T) {
			st, e := NewState()
			if e != nil {
				t.Fatal(e)
			}
			o := Options{Protocol: p, Host: "test.example.org", SNI: "test.example.org", Port: 33444, Cert: "cert.pem", Key: "key.pem", RealityTarget: "test.example.org:443"}
			a := Client{"a", "one"}
			b := Client{"b", "two"}
			first, e := GenerateFor(o, &st, []Client{a, b})
			if e != nil {
				t.Fatal(e)
			}
			second, e := GenerateFor(o, &st, []Client{b})
			if e != nil {
				t.Fatal(e)
			}
			if first.Clients[1] != second.Clients[0] || first.Clients[0] == first.Clients[1] {
				t.Fatal("per-user credentials are not stable and distinct")
			}
			changed, e := GenerateFor(o, &st, []Client{{"b", "changed"}})
			if e != nil || changed.Clients[0] == second.Clients[0] {
				t.Fatal("password change did not rotate credential")
			}
			u, e := url.Parse(first.Clients[0])
			if e != nil {
				t.Fatal(e)
			}
			if p == "mieru-udp" && u.Query().Get("protocol") != "UDP" {
				t.Fatal(u.Query())
			}
			if p == "tlsvision" && (u.Query().Get("flow") != "xtls-rprx-vision" || u.Query().Get("security") != "tls") {
				t.Fatal(u.Query())
			}
			if p == "xhttp-reality" && (u.Query().Get("type") != "xhttp" || u.Query().Get("security") != "reality" || u.Query().Get("pbk") == "") {
				t.Fatal(u.Query())
			}
			var doc map[string]any
			if json.Unmarshal(first.Server, &doc) != nil {
				t.Fatal("invalid JSON")
			}
			if p == "tuic" {
				in := doc["inbounds"].([]any)[0].(map[string]any)
				if in["zero_rtt_handshake"] != false {
					t.Fatal("0RTT enabled")
				}
			}
		})
	}
}
