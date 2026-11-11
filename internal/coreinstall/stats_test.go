package coreinstall

import "testing"

func TestParseStats(t *testing.T) {
	out := []byte(`{"stat":[
	 {"name":"user>>>alice>>>traffic>>>uplink","value":"100"},
	 {"name":"user>>>alice>>>traffic>>>downlink","value":"2500"},
	 {"name":"user>>>bob>>>traffic>>>downlink","value":7},
	 {"name":"inbound>>>api>>>traffic>>>uplink","value":"999"}]}`)
	got, err := parseStats(out)
	if err != nil {
		t.Fatal(err)
	}
	if got["alice"] != 2600 || got["bob"] != 7 || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got, err := parseStats([]byte(`{}`)); err != nil || len(got) != 0 {
		t.Fatalf("no stats must read as empty: %v %v", got, err)
	}
	if _, err := parseStats([]byte(`not json`)); err == nil {
		t.Fatal("garbage must be an error")
	}
}
