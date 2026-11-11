package coreplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAWGStatePeerIdentityAndPrivateKeySeparation(t *testing.T) {
	o := Options{Protocol: "amneziawg", Host: "2001:db8::1", Port: 51840, Subnet: "10.203.72.0/24", Egress: "eth0"}
	st, err := NewState()
	if err != nil {
		t.Fatal(err)
	}
	a, b := Client{"alice", "a"}, Client{"bob", "b"}
	one, err := GenerateFor(o, &st, []Client{a})
	if err != nil {
		t.Fatal(err)
	}
	two, err := GenerateFor(o, &st, []Client{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if one.Clients[0] != two.Clients[1] {
		t.Fatal("adding a peer changed an existing address or key")
	}
	if strings.Contains(one.Clients[0], st.ServerPrivate) || strings.Contains(one.Clients[0], st.Seed) {
		t.Fatal("server private state escaped into a client file")
	}
	for _, field := range []string{"HeaderProtectionKey = ", "ContentPaddingAddition = 0-32", "S4 = 32", "Endpoint = [2001:db8::1]:51840"} {
		if !strings.Contains(one.Clients[0], field) {
			t.Fatalf("missing supported AWG field %s", field)
		}
	}
	var manifest AWGServer
	if err := json.Unmarshal(two.Server, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Peers) != 2 || manifest.Peers[0].Address == manifest.Peers[1].Address {
		t.Fatal("peer addresses overlap")
	}
	a.Secret = "changed"
	changed, err := GenerateFor(o, &st, []Client{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Clients[0] == one.Clients[0] || changed.Clients[1] != two.Clients[0] {
		t.Fatal("password rotation did not isolate the changed peer")
	}
}
