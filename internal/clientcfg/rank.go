package clientcfg

import "sort"

// Output order. Configurations are listed from the fastest, most dependable
// way in to the slowest, so the first ones a client imports are the ones most
// people should try first. The order follows what works on a filtered
// network: TCP carrying ordinary TLS first (VLESS over WebSocket, XHTTP,
// REALITY, NaïveProxy, SSH over TLS, plain SSH), then the UDP-based and
// unencrypted ones, which depend on the network, and the DNS tunnels last:
// slow, but often the only thing that works during a shutdown.
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
	case "hysteria2":
		return 70
	case "wireguard":
		return 80
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
