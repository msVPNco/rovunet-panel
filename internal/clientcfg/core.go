package clientcfg

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

// GenerateCoreURI wraps an Xray core client profile (a vless://, hysteria2://
// link or a WireGuard file) in a native rovunet:// profile of type "xray",
// carrying the same collection as the user's other configurations so the app
// groups it with them. name is the profile name the app shows.
//
// The app reads such a profile as a common v29 base (parsed as type socks5)
// plus the core link in FCoreLink, and takes the endpoint from the link.
func GenerateCoreURI(name, link string, cfg *config.Config, opts URIOptions) (string, error) {
	if strings.TrimSpace(link) == "" {
		return "", fmt.Errorf("empty core profile")
	}
	base := &config.TunnelConfig{Tag: name, Transport: config.TransportSOCKS, Backend: config.BackendSOCKS}
	uri, err := GeneratePortableURI(base, cfg.GetBackend(config.BackendSOCKS), cfg, URIOptions{Username: opts.Username}, "rovunet")
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "rovunet://"))
	if err != nil {
		return "", err
	}
	fields := strings.Split(string(raw), "|")
	if len(fields) != TotalFieldsV33 {
		return "", fmt.Errorf("unexpected profile layout")
	}
	fields[FTunnelType] = "xray"
	fields[FName] = stripFieldDelimiter(name)
	fields[FCoreLink] = base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(link)))
	return "rovunet://" + base64.StdEncoding.EncodeToString([]byte(strings.Join(fields, "|"))), nil
}
