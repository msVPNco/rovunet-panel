package proxy

import (
	"bytes"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anonvector/slipgate/internal/certs"
)

// TestVLESSBehindTLSReverseProxy stands in for Caddy: a TLS-terminating
// reverse proxy hands the WebSocket upgrade to the plain local VLESS server.
func TestVLESSBehindTLSReverseProxy(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	target, _ := url.Parse("http://" + e.addr)
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.FlushInterval = -1

	dir := t.TempDir()
	cf, kf := filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	if err := certs.GenerateSelfSigned(cf, kf, "cdn.example.com"); err != nil {
		t.Fatal(err)
	}
	cert, err := tls.LoadX509KeyPair(cf, kf)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"http/1.1"}})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(testWSPath, rp)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("decoy")) })
	go http.Serve(ln, mux)
	t.Cleanup(func() { ln.Close() })

	front := *e
	front.addr = ln.Addr().String()
	front.tls = true
	host, port := startEchoTCP(t)
	c, status := dialWS(t, &front, testWSPath, "")
	if !strings.HasPrefix(status, "101") {
		t.Fatalf("upgrade through front: %s", status)
	}
	first := []byte("hello via front")
	if err := c.writeFrame(0x2, append(buildVLESSRequest(e.userID("alice", "pw-alice"), vlessCmdTCP, host, port), first...)); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, c, expectResponseHeader(t, c), len(first)); !bytes.Equal(got, first) {
		t.Fatalf("echo %q", got)
	}
	_ = net.IPv4len
}
