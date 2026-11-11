package handlers

import (
	"bytes"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
)

func TestApprovedTwentyFourNamesKeepSubscriptionIdentity(t *testing.T) {
	cfg, path := twentyProfileSubscriptionFixture(t)
	addManagedSubscriptionFixture(t, cfg)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	user := cfg.Users[0]
	androidURL, windowsURL := SubscriptionURL(cfg, user), WindowsSubscriptionURL(cfg, user)
	approved := map[string]string{
		"vless": "VLESS WS TLS", "xh": "VLESS XHTTP TLS", "reality": "REALITY 1", "reality2": "REALITY 2",
		"managed-trojan": "Trojan TLS", "managed-vlessenc": "VLESS Encryption",
		"naive-socks": "NaiveProxy · SOCKS5", "naive-ssh": "NaiveProxy · SSH", "stuntls": "StunTLS · SSH",
		"direct-ssh": "SSH · Direct", "hy2": "Hysteria 2", "wireguard": "WireGuard", "managed-awg": "AmneziaWG",
		"managed-ss2022": "Shadowsocks 2022", "soar": "Soar · Direct", "soar-ssh": "Soar · SSH",
		"dnstt-socks": "DNSTT · SOCKS5", "noizdns-socks": "NoizDNS · SOCKS5", "dnstt-ssh": "DNSTT · SSH", "noizdns-ssh": "NoizDNS · SSH",
		"vaydns-socks": "VayDNS · SOCKS5", "vaydns-ssh": "VayDNS · SSH", "slipstream-socks": "Slipstream · SOCKS5", "slipstream-ssh": "Slipstream · SSH",
	}
	for refresh := 0; refresh < 2; refresh++ {
		if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
			t.Fatal(err)
		}
		for client, count := range map[string]int{"android": 24, "windows": 22} {
			feed := savedSubscription(t, user, client)
			if len(feed.Items) != count || feed.Name != user.Username {
				t.Fatalf("%s subscription membership/name changed", client)
			}
			seen := map[string]bool{}
			for _, item := range feed.Items {
				name, ok := approved[item.ID]
				if !ok || seen[item.ID] {
					t.Fatalf("unexpected or duplicate profile ID %s", item.ID)
				}
				seen[item.ID] = true
				raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(item.Link, "rovunet://"))
				if err != nil {
					t.Fatal("invalid native profile")
				}
				fields := strings.Split(string(raw), "|")
				if len(fields) != clientcfg.TotalFieldsV33 || fields[clientcfg.FName] != name {
					t.Fatalf("wrong display name for %s", item.ID)
				}
			}
			if client == "windows" && (seen["managed-awg"] || seen["managed-ss2022"]) {
				t.Fatal("Windows membership changed")
			}
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("display names changed persisted account/tunnel configuration")
		}
		if SubscriptionURL(cfg, user) != androidURL || WindowsSubscriptionURL(cfg, user) != windowsURL {
			t.Fatal("existing subscription URL changed")
		}
	}
}
