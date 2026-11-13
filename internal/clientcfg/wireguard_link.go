package clientcfg

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// WireGuardClientURI carries the generated single-peer AWG file losslessly.
// A URI also lets old Windows readers report only the unsupported scheme,
// instead of accidentally including a private config file in their summary.
func WireGuardClientURI(name, document string) (string, error) {
	sections := []string{"[Interface]", "[Peer]"}
	allowed := []map[string]bool{
		{"privatekey": true, "address": true, "dns": true, "mtu": true,
			"jc": true, "jmin": true, "jmax": true, "s1": true, "s2": true, "s3": true, "s4": true,
			"h1": true, "h2": true, "h3": true, "h4": true, "headerprotectionkey": true, "contentpaddingaddition": true},
		{"publickey": true, "endpoint": true, "allowedips": true, "presharedkey": true, "persistentkeepalive": true},
	}
	values := url.Values{}
	section := -1
	for _, raw := range strings.Split(document, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section++
			if section >= len(sections) || line != sections[section] {
				return "", fmt.Errorf("requires one generated single-peer configuration")
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
		if !ok || section < 0 || !allowed[section][key] || values.Has(key) || value == "" {
			return "", fmt.Errorf("unsupported or duplicate generated configuration field")
		}
		values.Set(key, value)
	}
	for _, key := range []string{"privatekey", "address", "publickey", "endpoint", "allowedips"} {
		if values.Get(key) == "" {
			return "", fmt.Errorf("incomplete generated configuration")
		}
	}
	endpoint, private := values.Get("endpoint"), values.Get("privatekey")
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		return "", fmt.Errorf("invalid generated endpoint")
	}
	values.Del("endpoint")
	values.Del("privatekey")
	return (&url.URL{Scheme: "wireguard", User: url.User(private), Host: endpoint, RawQuery: values.Encode(), Fragment: name}).String(), nil
}
