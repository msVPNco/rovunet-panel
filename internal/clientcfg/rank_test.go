package clientcfg

import "testing"

func TestRankOrder(t *testing.T) {
	order := []int{
		CoreRank("tlsvision"), CoreRank("reality"), CoreRank("xhttp-reality"), CoreRank("xhttp"),
		CoreRank("hysteria2"), CoreRank("tuic"), CoreRank("mieru-tcp"), CoreRank("mieru-udp"),
		CoreRank("anytls"), CoreRank("ss2022"), CoreRank("vlessenc"), CoreRank("trojan"),
		CoreRank("amneziawg"), CoreRank("wireguard"), TunnelRank("naive"), TunnelRank("vless"),
		TunnelRank("stuntls"), TunnelRank("direct-ssh"), TunnelRank("direct-socks5"),
		TunnelRank("soar"), TunnelRank("dnstt"), TunnelRank("vaydns"), TunnelRank("slipstream"),
	}
	for i := 1; i < len(order); i++ {
		if order[i-1] >= order[i] {
			t.Fatalf("rank %d (%d) is not before rank %d (%d)", i-1, order[i-1], i, order[i])
		}
	}
	if TunnelRank("unknown") >= TunnelRank("soar") || CoreRank("unknown") >= TunnelRank("soar") {
		t.Fatal("DNS transports must remain at the end, including with an unknown non-DNS protocol")
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
