package clientcfg

import "sort"

// Output order is a fixed presentation policy. It is not a measurement of
// latency, throughput or reliability on the user's network.
//
// Ranks are spaced so a new protocol can be slotted in between.

// TunnelRank is the position of a tunnel transport in the output.
func TunnelRank(transport string) int {
	switch transport {
	case "vless":
		return 160
	case "naive":
		return 150
	case "stuntls":
		return 300
	case "direct-ssh":
		return 310
	case "direct-socks5":
		return 320
	case "soar":
		return 1000
	case "dnstt":
		return 1010
	case "vaydns":
		return 1020
	case "slipstream":
		return 1030
	}
	return 900
}

// CoreRank is the position of an Xray core protocol in the output.
func CoreRank(protocol string) int {
	switch protocol {
	case "tlsvision":
		return 10
	case "reality":
		return 20
	case "xhttp-reality":
		return 30
	case "xhttp":
		return 40
	case "hysteria2":
		return 50
	case "tuic":
		return 60
	case "mieru-tcp":
		return 70
	case "mieru-udp":
		return 80
	case "anytls":
		return 90
	case "ss2022":
		return 100
	case "vlessenc":
		return 110
	case "trojan":
		return 120
	case "amneziawg":
		return 130
	case "wireguard":
		return 140
	}
	return 900
}

// SortByRank returns a copy of items ordered by rank, keeping the original
// order among items of equal rank.
func SortByRank[T any](items []T, rank func(T) int) []T {
	out := append([]T(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}
