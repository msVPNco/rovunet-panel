package coreplan

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func shadowsocksLinkParts(t *testing.T, link string) (string, []string) {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "ss" || u.User == nil {
		t.Fatalf("invalid Shadowsocks URI: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(u.User.Username())
	if err != nil {
		t.Fatal(err)
	}
	method, password, ok := strings.Cut(string(raw), ":")
	if !ok {
		t.Fatal("missing Shadowsocks credentials")
	}
	return method, strings.Split(password, ":")
}

func TestShadowsocksVariantsUseMatchingServerAndClientCredentials(t *testing.T) {
	for _, tc := range []struct {
		protocol, method string
		keyBytes, keys   int
	}{
		{"ss2022", "2022-blake3-aes-256-gcm", 32, 2},
		{"ss2022-aes128", "2022-blake3-aes-128-gcm", 16, 2},
		{"ss-aead-chacha20", "chacha20-ietf-poly1305", 32, 1},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			st, err := NewState()
			if err != nil {
				t.Fatal(err)
			}
			clients := []Client{{Name: "alice", Secret: "fixture-a"}, {Name: "bob", Secret: "fixture-b"}}
			b, err := GenerateFor(Options{Protocol: tc.protocol, Host: "example.test", Port: 51820}, &st, clients)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Inbounds []struct {
					Protocol string `json:"protocol"`
					Settings struct {
						Method, Password, Network string
						Clients                   []struct{ Method, Password, Email string }
					} `json:"settings"`
				} `json:"inbounds"`
			}
			if err := json.Unmarshal(b.Server, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Inbounds) != 1 || doc.Inbounds[0].Protocol != "shadowsocks" {
				t.Fatal("missing Shadowsocks inbound")
			}
			settings := doc.Inbounds[0].Settings
			if settings.Network != "tcp,udp" || len(settings.Clients) != 2 || len(b.Clients) != 2 {
				t.Fatal("TCP/UDP or independent clients missing")
			}
			if tc.keys == 2 {
				if settings.Method != tc.method || settings.Password == "" {
					t.Fatal("SS2022 server key is missing")
				}
			} else if settings.Method != "" || settings.Password != "" {
				t.Fatal("classic AEAD must use per-user methods and passwords")
			}
			for i, link := range b.Clients {
				method, keys := shadowsocksLinkParts(t, link)
				if method != tc.method || len(keys) != tc.keys {
					t.Fatalf("wrong client cipher or key count: %s/%d", method, len(keys))
				}
				for _, key := range keys {
					raw, err := base64.StdEncoding.DecodeString(key)
					if err != nil || len(raw) != tc.keyBytes {
						t.Fatalf("wrong key length: %d, %v", len(raw), err)
					}
				}
				user := settings.Clients[i]
				if user.Email != clients[i].Name || user.Password != keys[len(keys)-1] {
					t.Fatal("client key does not match its own server account")
				}
				if tc.keys == 2 && (keys[0] != settings.Password || user.Method != "") {
					t.Fatal("SS2022 server/user key layout changed")
				}
				if tc.keys == 1 && user.Method != tc.method {
					t.Fatal("classic AEAD client method is missing")
				}
			}
			if settings.Clients[0].Password == settings.Clients[1].Password {
				t.Fatal("different users share a key")
			}
		})
	}
}

func TestLegacyShadowsocks2022CredentialsRemainUnchanged(t *testing.T) {
	st, err := NewState()
	if err != nil {
		t.Fatal(err)
	}
	st.Seed = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	b, err := GenerateFor(Options{Protocol: "ss2022", Host: "example.test", Port: 28444}, &st, []Client{{Name: "alice", Secret: "fixture-a"}})
	if err != nil {
		t.Fatal(err)
	}
	method, keys := shadowsocksLinkParts(t, b.Clients[0])
	// Fixed vectors from the original v2.0.2 HMAC scopes and full 32-byte keys.
	if method != "2022-blake3-aes-256-gcm" || len(keys) != 2 ||
		keys[0] != "t2fWElHgqQcXC9DWfZFi0lRirJtBymoJY7WUXWMcMPQ=" ||
		keys[1] != "fTx8x8e1GYHR6ORjlRMBaeyjc+ldk9htqmuxURmkdmg=" {
		t.Fatal("existing AES-256 subscription credentials changed")
	}
}
