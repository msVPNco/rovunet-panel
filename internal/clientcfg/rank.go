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
		return 10
	case "naive":
		return 40
	case "stuntls":
		return 50
	case "direct-ssh":
		return 60
	case "direct-socks5":
		return 90
	case "soar":
		return 100
	case "dnstt":
		return 110
	case "vaydns":
		return 120
	case "slipstream":
		return 130
	}
	return 200
}

// CoreRank is the position of an Xray core protocol in the output.
func CoreRank(protocol string) int {
	switch protocol {
	case "xhttp":
		return 20
	case "reality":
		return 30
	case "trojan":
		return 35
	case "vlessenc":
		return 37
	case "hysteria2":
		return 70
	case "wireguard":
		return 80
	case "amneziawg":
		return 82
	case "ss2022":
		return 85
	}
	return 85
}

// SortByRank returns a copy of items ordered by rank, keeping the original
// order among items of equal rank.
func SortByRank[T any](items []T, rank func(T) int) []T {
	out := append([]T(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}
