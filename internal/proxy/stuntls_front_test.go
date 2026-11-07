package proxy

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/certs"
)

// TestStunTLSBehindTLSReverseProxy stands in for Caddy reaching StunTLS
// over TLS (tls_insecure_skip_verify) and passing the WebSocket upgrade.
func TestStunTLSBehindTLSReverseProxy(t *testing.T) {
	dir := t.TempDir()
	cf, kf := filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	if err := certs.GenerateSelfSigned(cf, kf, "stuntls.local"); err != nil {
		t.Fatal(err)
	}
	cert, _ := tls.LoadX509KeyPair(cf, kf)

	// SSH stand-in: sends a banner, then echoes
	sshLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer sshLn.Close()
	go func() {
		for {
			c, err := sshLn.Accept()
			if err != nil {
				return
			}
			go func() {
				c.Write([]byte("SSH-2.0-test\r\n"))
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						c.Close()
						return
					}
					c.Write(buf[:n])
				}
			}()
		}
	}()

	// StunTLS on TLS
	stLn, _ := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	defer stLn.Close()
	st := &StunTLSServer{sshAddr: sshLn.Addr().String()}
	go func() {
		for {
			c, err := stLn.Accept()
			if err != nil {
				return
			}
			go st.handleConn(c)
		}
	}()

	// Front: TLS reverse proxy for /ws/abc → https://stuntls
	target, _ := url.Parse("https://" + stLn.Addr().String())
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = -1
	rp.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	mux := http.NewServeMux()
	mux.Handle("/ws/abc", rp)
	frontLn, _ := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"http/1.1"}})
	defer frontLn.Close()
	go http.Serve(frontLn, mux)

	conn, err := tls.Dial("tcp", frontLn.Addr().String(), &tls.Config{InsecureSkipVerify: true, ServerName: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	conn.Write([]byte("GET /ws/abc HTTP/1.1\r\nHost: example.com\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n"))
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != 101 {
		t.Fatalf("upgrade: %v %v", err, resp)
	}
	op, p, err := readWSFrame(br)
	if err != nil || op != 0x2 || !strings.HasPrefix(string(p), "SSH-2.0-test") {
		t.Fatalf("banner: op=%x %q %v", op, p, err)
	}
	conn.Write(clientFrame(0x9, []byte("k")))
	conn.Write(clientFrame(0x2, []byte("ping-data")))
	got, sawPong := "", false
	for !(sawPong && got == "ping-data") {
		op, p, err := readWSFrame(br)
		if err != nil {
			t.Fatalf("read: %v (pong=%v got=%q)", err, sawPong, got)
		}
		if op == 0xA {
			sawPong = true
		} else if op == 0x2 {
			got += string(p)
		}
	}
}
