package clientcfg

import "testing"

func TestRankOrder(t *testing.T) {
	order := []int{
		TunnelRank("vless"), CoreRank("xhttp"), CoreRank("reality"), TunnelRank("naive"),
		TunnelRank("stuntls"), TunnelRank("direct-ssh"), CoreRank("hysteria2"), CoreRank("wireguard"),
		TunnelRank("direct-socks5"), TunnelRank("soar"), TunnelRank("dnstt"), TunnelRank("vaydns"), TunnelRank("slipstream"),
	}
	for i := 1; i < len(order); i++ {
		if order[i-1] >= order[i] {
			t.Fatalf("rank %d (%d) is not before rank %d (%d)", i-1, order[i-1], i, order[i])
		}
	}
	if TunnelRank("unknown") <= TunnelRank("slipstream") {
		t.Fatal("unknown transports must come last")
	}
}

func TestSortByRankIsStable(t *testing.T) {
	type item struct {
		name string
		r    int
	}
	in := []item{{"b1", 2}, {"a1", 1}, {"b2", 2}, {"a2", 1}}
	got := SortByRank(in, func(i item) int { return i.r })
	want := []string{"a1", "a2", "b1", "b2"}
	for i := range want {
		if got[i].name != want[i] {
			t.Fatalf("got %v", got)
		}
	}
	if in[0].name != "b1" {
		t.Fatal("input modified")
	}
}
