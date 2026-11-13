package coreplan

import (
	"bytes"
	"encoding/json"
	"net/url"
	"testing"
)

func TestHysteriaSalamanderPreservesIdentityAndSurvivesUserChanges(t *testing.T) {
	o := Options{Protocol: "hysteria2", Host: "example.org", SNI: "example.org", Port: 8443, Cert: "cert.pem", Key: "key.pem"}
	st, err := NewState()
	if err != nil {
		t.Fatal(err)
	}
	users := []Client{{Name: "alice", Secret: "fixture-a"}, {Name: "bob", Secret: "fixture-b"}}
	plain, err := GenerateFor(o, &st, users)
	if err != nil {
		t.Fatal(err)
	}
	o.HysteriaSalamander = true
	masked, err := GenerateFor(o, &st, users)
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(masked.Server, &tree); err != nil {
		t.Fatal(err)
	}
	stream := tree["inbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
	mask := stream["finalmask"].(map[string]any)["udp"].([]any)[0].(map[string]any)
	password := mask["settings"].(map[string]any)["password"].(string)
	if mask["type"] != "salamander" || len(password) < 32 {
		t.Fatal("missing server mask")
	}
	for i, raw := range masked.Clients {
		old, _ := url.Parse(plain.Clients[i])
		next, _ := url.Parse(raw)
		if old.User.String() != next.User.String() || old.Host != next.Host || old.Fragment != next.Fragment {
			t.Fatal("identity changed")
		}
		q := next.Query()
		if q.Get("obfs") != "salamander" || q.Get("obfs-password") != password {
			t.Fatal("server/client mask mismatch")
		}
		q.Del("obfs")
		q.Del("obfs-password")
		next.RawQuery = q.Encode()
		if next.String() != old.String() {
			t.Fatal("unrelated link field changed")
		}
	}
	users = append(users, Client{Name: "carol", Secret: "fixture-c"})
	regenerated, err := GenerateFor(o, &st, users)
	if err != nil || regenerated.Clients[0] != masked.Clients[0] {
		t.Fatal("mask changed on account addition", err)
	}
	o.HysteriaSalamander = false
	restored, err := GenerateFor(o, &st, users[:2])
	if err != nil || !bytes.Equal(restored.Server, plain.Server) || restored.Clients[0] != plain.Clients[0] {
		t.Fatal("plain rollback changed identity", err)
	}
	o.Protocol = "wireguard"
	o.HysteriaSalamander = true
	if _, err := GenerateFor(o, &st, users); err == nil {
		t.Fatal("mask accepted for unrelated protocol")
	}
}
