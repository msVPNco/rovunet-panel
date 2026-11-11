// Package protocols defines the account capabilities shared by provisioning
// and every export surface. Counting traffic and enforcing a quota are
// separate capabilities.
package protocols

func Metered(protocol string) bool {
	switch protocol {
	case "vless", "reality", "xhttp", "ss2022", "trojan", "vlessenc":
		return true
	}
	return false
}

// QuotaEligible preserves the existing quota paths. The pinned Xray counts
// the new protocols, but HandlerService removal leaves established streams
// alive. They must not provide a bypass for a traffic-limited account.
func QuotaEligible(protocol string) bool {
	switch protocol {
	case "vless", "reality", "xhttp":
		return true
	}
	return false
}

func Allowed(protocol string, trafficLimited, expires bool) bool {
	if trafficLimited && !QuotaEligible(protocol) {
		return false
	}
	// Direct Soar has a shared server key and no per-account authentication.
	// Its access cannot be revoked when a timed account expires.
	return protocol != "soar-direct" || !expires
}
