package coreplan

import (
	"math/rand"
	"strings"
	"testing"
)

func TestPickRealityTarget(t *testing.T) {
	targets := []string{"a.example", "b.example", "c.example"}
	only := func(h string) bool { return h == "b.example" }
	got, err := PickRealityTarget(targets, only, rand.New(rand.NewSource(1)))
	if err != nil || got != "b.example" {
		t.Fatalf("got %q, %v; only b passes", got, err)
	}
	seen := map[string]bool{}
	rnd := rand.New(rand.NewSource(7))
	for i := 0; i < 60; i++ {
		h, err := PickRealityTarget(targets, func(string) bool { return true }, rnd)
		if err != nil {
			t.Fatal(err)
		}
		seen[h] = true
	}
	if len(seen) != 3 {
		t.Fatalf("a random pick must reach every working target, saw %v", seen)
	}
	if _, err := PickRealityTarget(targets, func(string) bool { return false }, rnd); err == nil || !strings.Contains(err.Error(), "--target") {
		t.Fatalf("no working target must fail with advice, got %v", err)
	}
}

func TestAutoRealityKeepsWhatWasGiven(t *testing.T) {
	o := Options{Protocol: "reality", RealityTarget: "www.example.com:443"}
	if h, err := AutoReality(&o); h != "" || err != nil || o.RealityTarget != "www.example.com:443" {
		t.Fatalf("a given target must be kept: %q %v %+v", h, err, o)
	}
	o = Options{Protocol: "xhttp"}
	if h, err := AutoReality(&o); h != "" || err != nil || o.RealityTarget != "" {
		t.Fatalf("other protocols are untouched: %q %v %+v", h, err, o)
	}
}

func TestRealityBankIsValid(t *testing.T) {
	seen := map[string]bool{}
	for _, h := range RealityTargets {
		if !validHost(h) || seen[h] {
			t.Errorf("bad or duplicate target %q", h)
		}
		seen[h] = true
	}
}

func TestMeteredCoresExposeStats(t *testing.T) {
	st, err := NewState()
	if err != nil {
		t.Fatal(err)
	}
	st.APIPort = 10085
	o := Options{Protocol: "reality", Host: "h.example.com", SNI: "www.microsoft.com", RealityTarget: "www.microsoft.com:443", Port: 9443}
	b, err := GenerateFor(o, &st, []Client{{Name: "alice", Secret: "s"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"email": "alice"`, `"StatsService"`, `"statsUserUplink": true`, `"dokodemo-door"`, `"port": 10085`, `"outboundTag": "api"`} {
		if !strings.Contains(string(b.Server), want) {
			t.Errorf("server config lacks %s:\n%s", want, b.Server)
		}
	}
	// No API port, no API: WireGuard and a core without a port stay as they were.
	st.APIPort = 0
	b, _ = GenerateFor(o, &st, []Client{{Name: "alice", Secret: "s"}})
	if strings.Contains(string(b.Server), "StatsService") || strings.Contains(string(b.Server), "dokodemo") {
		t.Fatalf("no statistics API without a port:\n%s", b.Server)
	}
	if Metered("wireguard") || Metered("hysteria2") || !Metered("xhttp") || !Metered("reality") {
		t.Fatal("only REALITY and XHTTP are metered")
	}
}
