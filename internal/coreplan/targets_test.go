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

func TestTargetFamily(t *testing.T) {
	same := [][2]string{
		{"www.microsoft.com", "learn.microsoft.com"},
		{"www.amazon.com", "m.media-amazon.com"},
		{"www.amazon.com", "images-na.ssl-images-amazon.com"},
		{"dl.google.com", "fonts.gstatic.com"},
		{"dl.google.com", "www.googletagmanager.com"},
		{"addons.mozilla.org", "www.mozilla.org:443"},
	}
	for _, p := range same {
		if TargetFamily(p[0]) != TargetFamily(p[1]) {
			t.Errorf("%s and %s are one company", p[0], p[1])
		}
	}
	if TargetFamily("www.apple.com") == TargetFamily("www.microsoft.com") || TargetFamily("www.nvidia.com") == TargetFamily("www.intel.com") {
		t.Fatal("different companies must differ")
	}
	// Every built-in site belongs to some family, and the bank has several.
	fams := map[string]bool{}
	for _, h := range RealityTargets {
		fams[TargetFamily(h)] = true
	}
	if len(fams) < 6 {
		t.Fatalf("the bank needs several companies, has %v", fams)
	}
}

// The second REALITY core never uses the first one's site or company.
func TestAutoRealityOtherAvoidsTheFirstCompany(t *testing.T) {
	old := realityCheck
	realityCheck = func(string) bool { return true }
	defer func() { realityCheck = old }()
	for _, first := range []string{"www.microsoft.com", "www.amazon.com", "dl.google.com", "www.apple.com"} {
		for i := 0; i < 200; i++ {
			o := Options{Protocol: "reality"}
			h, err := AutoRealityOther(&o, first)
			if err != nil {
				t.Fatal(err)
			}
			if TargetFamily(h) == TargetFamily(first) {
				t.Fatalf("first %s, second %s: same company", first, h)
			}
			if o.RealityTarget != h+":443" || o.SNI != h {
				t.Fatalf("target and name not set: %+v", o)
			}
		}
	}
	// Only the first company's sites answer: no second disguise, with advice.
	realityCheck = func(h string) bool { return TargetFamily(h) == "microsoft" }
	o := Options{Protocol: "reality"}
	if _, err := AutoRealityOther(&o, "www.microsoft.com"); err == nil {
		t.Fatal("no other company works: must fail, not repeat the first")
	}
}

// Two cores of one protocol give a user links with different names.
func TestLinkNamesTellTwoCoresApart(t *testing.T) {
	st, err := NewState()
	if err != nil {
		t.Fatal(err)
	}
	link := func(core string) string {
		o := Options{Protocol: "reality", Host: "h.example.com", SNI: "www.apple.com", RealityTarget: "www.apple.com:443", Port: 2087, CoreName: core}
		b, err := GenerateFor(o, &st, []Client{{Name: "alice", Secret: "s"}})
		if err != nil {
			t.Fatal(err)
		}
		return b.Clients[0]
	}
	if l := link("reality"); !strings.HasSuffix(l, "#Rovunet-reality-alice") {
		t.Fatalf("first core keeps its name: %s", l)
	}
	if l := link("reality2"); !strings.HasSuffix(l, "#Rovunet-reality2-alice") {
		t.Fatalf("second core is named apart: %s", l)
	}
	if l := link("my-core"); !strings.HasSuffix(l, "#Rovunet-reality-alice") {
		t.Fatalf("a name that does not extend the protocol changes nothing: %s", l)
	}
	if l := link(""); !strings.HasSuffix(l, "#Rovunet-reality-alice") {
		t.Fatalf("no core name changes nothing: %s", l)
	}
}
