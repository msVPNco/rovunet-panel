package coreinstall

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

// Use a port free for both transports, as every generated Shadowsocks server
// binds TCP and UDP. Holding the sockets together also avoids selecting a TCP
// port already occupied by an unrelated UDP listener in the test namespace.
func freeShadowsocksPort(t *testing.T) int {
	t.Helper()
	for i := 0; i < 20; i++ {
		tcp, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := tcp.Addr().(*net.TCPAddr).Port
		udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		_ = tcp.Close()
		if err == nil {
			_ = udp.Close()
			return port
		}
	}
	t.Fatal("no free TCP/UDP port for Shadowsocks")
	return 0
}

// This exercises the actual generated credentials against the exact reviewed
// engine. An isolated loopback UDP echo is the only destination; no public
// network or phone/device compatibility is implied by this test.
func TestShadowsocksUDPPayload(t *testing.T) {
	engine := os.Getenv("XRAY")
	if engine == "" {
		t.Skip("set XRAY to the exact reviewed Xray executable")
	}
	if err := verifyGoSource(engine, XrayCommit); err != nil {
		t.Fatalf("unreviewed runtime fixture: %v", err)
	}
	oldRoot, oldXray := Root, XrayPath
	t.Cleanup(func() { Root, XrayPath = oldRoot, oldXray })
	Root, XrayPath = t.TempDir(), engine
	echo, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 65535)
		for {
			n, peer, err := echo.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = echo.WriteToUDP(buf[:n], peer)
		}
	}()
	t.Cleanup(func() { _ = echo.Close(); <-done })
	echoPort := echo.LocalAddr().(*net.UDPAddr).Port
	for _, protocol := range []string{"ss2022", "ss2022-aes128", "ss-aead-chacha20"} {
		t.Run(protocol, func(t *testing.T) {
			st, err := coreplan.NewState()
			if err != nil {
				t.Fatal(err)
			}
			serverPort := freeShadowsocksPort(t)
			bundle, err := coreplan.GenerateFor(coreplan.Options{Protocol: protocol, Host: "127.0.0.1", Port: serverPort}, &st,
				[]coreplan.Client{{Name: "alice", Secret: "udp-a"}, {Name: "bob", Secret: "udp-b"}})
			if err != nil {
				t.Fatal(err)
			}
			var server map[string]any
			if err := json.Unmarshal(bundle.Server, &server); err != nil {
				t.Fatal(err)
			}
			// The pinned freedom outbound blocks loopback by default. Change only
			// the test destination policy, retaining the production inbound/auth.
			server["outbounds"].([]any)[0].(map[string]any)["settings"] = map[string]any{
				"finalRules": []any{map[string]any{"action": "allow", "ip": []string{"127.0.0.1/32"}}},
			}
			data, err := json.Marshal(server)
			if err != nil {
				t.Fatal(err)
			}
			runtimeProcess(t, filepath.Join(t.TempDir(), "server.json"), data, "", serverPort)
			for i, link := range bundle.Clients {
				u, err := url.Parse(link)
				if err != nil || u.Scheme != "ss" || u.User == nil {
					t.Fatal("invalid generated Shadowsocks client")
				}
				auth, err := base64.RawURLEncoding.DecodeString(u.User.Username())
				if err != nil {
					t.Fatal(err)
				}
				method, password, ok := strings.Cut(string(auth), ":")
				if !ok {
					t.Fatal("missing generated Shadowsocks credentials")
				}
				port, err := strconv.Atoi(u.Port())
				if err != nil {
					t.Fatal(err)
				}
				udpPort, readyPort := freeShadowsocksPort(t), freeTCP(t)
				client := map[string]any{
					"inbounds": []any{
						map[string]any{"listen": "127.0.0.1", "port": udpPort, "protocol": "dokodemo-door", "settings": map[string]any{"address": "127.0.0.1", "port": echoPort, "network": "udp"}},
						map[string]any{"listen": "127.0.0.1", "port": readyPort, "protocol": "socks", "settings": map[string]any{"auth": "noauth"}},
					},
					"outbounds": []any{map[string]any{"protocol": "shadowsocks", "settings": map[string]any{"servers": []any{
						map[string]any{"address": u.Hostname(), "port": port, "method": method, "password": password},
					}}}},
				}
				data, err := json.Marshal(client)
				if err != nil {
					t.Fatal(err)
				}
				runtimeProcess(t, filepath.Join(t.TempDir(), "client.json"), data, "", readyPort)
				conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: udpPort})
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				payload := bytes.Repeat([]byte{byte(i + 1), 'u', 'd', 'p'}, 256)
				if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					t.Fatal(err)
				}
				if _, err := conn.Write(payload); err != nil {
					t.Fatal(err)
				}
				buf := make([]byte, 2048)
				n, err := conn.Read(buf)
				if err != nil || !bytes.Equal(payload, buf[:n]) {
					t.Fatalf("account %d UDP round trip failed: %v", i, err)
				}
			}
		})
	}
}
