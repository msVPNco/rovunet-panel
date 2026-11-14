package coreplan

func Warnings(protocol string) []string {
	const sessions = "Traffic-limited accounts are excluded until durable per-account traffic accounting is validated. Revocation restarts this shared compatibility core to close existing streams; other users may briefly reconnect."
	switch protocol {
	case "ss2022":
		return []string{sessions, "The pinned Xray deprecates Shadowsocks, citing lack of Forward Secrecy and possible future removal. This is an optional compatibility protocol."}
	case "trojan":
		return []string{sessions, "The pinned Xray deprecates Trojan without Flow and recommends VLESS with Flow & Seed. A successful connection does not guarantee future engine support."}
	case "vlessenc":
		return []string{sessions}
	case "amneziawg":
		return []string{"AmneziaWG is available for unlimited and time-limited accounts. Traffic-limited accounts are excluded until peer accounting and crash recovery are validated."}
	}
	return nil
}
