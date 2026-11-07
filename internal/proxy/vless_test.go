package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/certs"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/keys"
)

const testWSPath = "/t3st-path"

type vlessTestEnv struct {
	srv       *VLESSServer
	addr      string
	secret    []byte
	usersFile string
	tls       bool
}

func writeUsers(t *testing.T, path string, users ...config.UserConfig) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"users": users})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func startVLESS(t *testing.T, useTLS, allowPrivate bool, mutate func(*VLESSOptions)) *vlessTestEnv {
	t.Helper()
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "vless.secret")
	if err := keys.GenerateVLESSSecret(secretFile); err != nil {
		t.Fatal(err)
	}
	secret, err := keys.LoadVLESSSecret(secretFile)
	if err != nil {
		t.Fatal(err)
	}
	usersFile := filepath.Join(dir, "config.json")
	writeUsers(t, usersFile, config.UserConfig{Username: "alice", Password: "pw-alice"})

	opts := VLESSOptions{
		Path:         testWSPath,
		SecretFile:   secretFile,
		UsersFile:    usersFile,
		BlockPorts:   []int{25},
		AllowPrivate: allowPrivate,
	}
	if useTLS {
		opts.CertFile = filepath.Join(dir, "cert.pem")
		opts.KeyFile = filepath.Join(dir, "key.pem")
		if err := certs.GenerateSelfSigned(opts.CertFile, opts.KeyFile, "cdn.example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if mutate != nil {
		mutate(&opts)
	}
	srv, err := NewVLESSServer(opts)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Serve(ctx, ln)
	t.Cleanup(cancel)
	return &vlessTestEnv{srv: srv, addr: ln.Addr().String(), secret: secret, usersFile: usersFile, tls: useTLS}
}

func (e *vlessTestEnv) userID(user, pass string) [16]byte {
	return keys.VLESSUserID(e.secret, user, pass)
}

// --- a client that behaves like SlipNet's VlessBridge ---

type testWSClient struct {
	conn net.Conn
	br   *bufio.Reader
}

func dialWS(t *testing.T, e *vlessTestEnv, path string, extraHeaders string) (*testWSClient, string) {
	t.Helper()
	var conn net.Conn
	var err error
	if e.tls {
		conn, err = tls.Dial("tcp", e.addr, &tls.Config{InsecureSkipVerify: true, ServerName: "cdn.example.com"})
	} else {
		conn, err = net.Dial("tcp", e.addr)
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: cdn.example.com\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		extraHeaders +
		"\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusSwitchingProtocols {
		if got, want := resp.Header.Get("Sec-WebSocket-Accept"), computeAcceptKey(key); got != want {
			t.Fatalf("Sec-WebSocket-Accept = %q, want %q", got, want)
		}
	}
	return &testWSClient{conn: conn, br: br}, resp.Status + "|" + resp.Header.Get("Sec-WebSocket-Protocol")
}

// writeFrame sends a masked frame, as a WebSocket client must.
func (c *testWSClient) writeFrame(opcode byte, payload []byte) error {
	var hdr []byte
	hdr = append(hdr, 0x80|opcode)
	n := len(payload)
	switch {
	case n <= 125:
		hdr = append(hdr, 0x80|byte(n))
	case n <= 0xFFFF:
		hdr = append(hdr, 0x80|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 0x80|127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	var mask [4]byte
	rand.Read(mask[:])
	hdr = append(hdr, mask[:]...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i&3]
	}
	_, err := c.conn.Write(append(hdr, masked...))
	return err
}

// readFrame returns the next frame, answering pings like VlessBridge does.
func (c *testWSClient) readFrame() (byte, []byte, error) {
	for {
		var hdr [2]byte
		if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
			return 0, nil, err
		}
		if hdr[1]&0x80 != 0 {
			return 0, nil, errors.New("server frame is masked")
		}
		op := hdr[0] & 0x0F
		n := uint64(hdr[1] & 0x7F)
		switch n {
		case 126:
			var ext [2]byte
			io.ReadFull(c.br, ext[:])
			n = uint64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			io.ReadFull(c.br, ext[:])
			n = binary.BigEndian.Uint64(ext[:])
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(c.br, p); err != nil {
			return 0, nil, err
		}
		if op == 0x9 {
			c.writeFrame(0xA, p)
			continue
		}
		return op, p, nil
	}
}

func buildVLESSRequest(id [16]byte, cmd byte, host string, port uint16) []byte {
	var b bytes.Buffer
	b.WriteByte(0)
	b.Write(id[:])
	b.WriteByte(0) // no addons
	b.WriteByte(cmd)
	binary.Write(&b, binary.BigEndian, port)
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			b.WriteByte(vlessAddrIPv4)
			b.Write(v4)
		} else {
			b.WriteByte(vlessAddrIPv6)
			b.Write(ip.To16())
		}
	} else {
		b.WriteByte(vlessAddrDomain)
		b.WriteByte(byte(len(host)))
		b.WriteString(host)
	}
	return b.Bytes()
}

func startEchoTCP(t *testing.T) (string, uint16) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	a := ln.Addr().(*net.TCPAddr)
	return a.IP.String(), uint16(a.Port)
}

// startBannerTCP is a server-speaks-first service, like sshd.
func startBannerTCP(t *testing.T, banner string) (string, uint16) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); c.Write([]byte(banner)); io.Copy(c, c) }()
		}
	}()
	a := ln.Addr().(*net.TCPAddr)
	return a.IP.String(), uint16(a.Port)
}

// expectResponseHeader reads the first frame and checks the VLESS response,
// exactly as VlessBridge.handleVlessWs does, returning any trailing data.
func expectResponseHeader(t *testing.T, c *testWSClient) []byte {
	t.Helper()
	op, p, err := c.readFrame()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if op == 0x8 {
		t.Fatalf("server closed the stream (code %v)", p)
	}
	if len(p) < 2 || p[0] != 0 {
		t.Fatalf("bad VLESS response %x", p)
	}
	return p[2+int(p[1]):]
}

func readUntil(t *testing.T, c *testWSClient, have []byte, want int) []byte {
	t.Helper()
	for len(have) < want {
		op, p, err := c.readFrame()
		if err != nil {
			t.Fatalf("read: %v (have %d of %d bytes)", err, len(have), want)
		}
		if op == 0x8 {
			t.Fatalf("closed early (have %d of %d bytes)", len(have), want)
		}
		have = append(have, p...)
	}
	return have
}

func TestVLESSTCPLikeSlipNet(t *testing.T) {
	for _, useTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("tls=%v", useTLS), func(t *testing.T) {
			e := startVLESS(t, useTLS, true, nil)
			host, port := startEchoTCP(t)
			c, status := dialWS(t, e, testWSPath, "")
			if !strings.HasPrefix(status, "101") {
				t.Fatalf("upgrade status %s", status)
			}
			// Header and first payload bundled in one frame, like VlessBridge
			first := []byte("hello through vless")
			req := append(buildVLESSRequest(e.userID("alice", "pw-alice"), vlessCmdTCP, host, port), first...)
			if err := c.writeFrame(0x2, req); err != nil {
				t.Fatal(err)
			}
			got := readUntil(t, c, expectResponseHeader(t, c), len(first))
			if !bytes.Equal(got, first) {
				t.Fatalf("echo = %q", got)
			}
			// A large write spans many server frames
			big := make([]byte, 300<<10)
			rand.Read(big)
			for off := 0; off < len(big); off += 16 << 10 {
				if err := c.writeFrame(0x2, big[off:min(off+16<<10, len(big))]); err != nil {
					t.Fatal(err)
				}
			}
			if got := readUntil(t, c, nil, len(big)); !bytes.Equal(got, big) {
				t.Fatal("large echo mismatch")
			}
			// Client ping (VlessBridge cover traffic) gets a pong
			if err := c.writeFrame(0x9, []byte("cover")); err != nil {
				t.Fatal(err)
			}
			op, p, err := c.readFrame()
			if err != nil || op != 0xA || string(p) != "cover" {
				t.Fatalf("pong: op=%x p=%q err=%v", op, p, err)
			}
		})
	}
}

func TestVLESSServerSpeaksFirst(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	host, port := startBannerTCP(t, "SSH-2.0-OpenSSH_9.6\r\n")
	c, _ := dialWS(t, e, testWSPath, "")
	// Header alone, no payload: the client waits for the server
	if err := c.writeFrame(0x2, buildVLESSRequest(e.userID("alice", "pw-alice"), vlessCmdTCP, host, port)); err != nil {
		t.Fatal(err)
	}
	got := readUntil(t, c, expectResponseHeader(t, c), len("SSH-2.0-OpenSSH_9.6\r\n"))
	if string(got) != "SSH-2.0-OpenSSH_9.6\r\n" {
		t.Fatalf("banner = %q", got)
	}
}

func TestVLESSUnknownUserRejected(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	host, port := startEchoTCP(t)
	c, _ := dialWS(t, e, testWSPath, "")
	c.writeFrame(0x2, buildVLESSRequest(e.userID("alice", "wrong"), vlessCmdTCP, host, port))
	op, _, err := c.readFrame()
	if err == nil && op != 0x8 {
		t.Fatalf("expected close, got opcode %x", op)
	}
}

func TestVLESSUsersReload(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	host, port := startEchoTCP(t)
	try := func(user, pass string) bool {
		c, _ := dialWS(t, e, testWSPath, "")
		defer c.conn.Close()
		c.writeFrame(0x2, append(buildVLESSRequest(e.userID(user, pass), vlessCmdTCP, host, port), 'x'))
		op, p, err := c.readFrame()
		return err == nil && op == 0x2 && len(p) >= 2 && p[0] == 0
	}
	if try("bob", "pw-bob") {
		t.Fatal("bob accepted before being added")
	}
	writeUsers(t, e.usersFile, config.UserConfig{Username: "bob", Password: "pw-bob"})
	if _, err := e.srv.ReloadUsers(); err != nil {
		t.Fatal(err)
	}
	if !try("bob", "pw-bob") {
		t.Fatal("bob rejected after reload")
	}
	if try("alice", "pw-alice") {
		t.Fatal("removed user alice still accepted")
	}
}

func TestVLESSReloadClosesRemovedUsersSessions(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	host, port := startEchoTCP(t)

	open := func(user, pass string) *testWSClient {
		c, _ := dialWS(t, e, testWSPath, "")
		t.Cleanup(func() { c.conn.Close() })
		c.writeFrame(0x2, append(buildVLESSRequest(e.userID(user, pass), vlessCmdTCP, host, port), 'x'))
		if op, p, err := c.readFrame(); err != nil || op != 0x2 || len(p) < 2 || p[0] != 0 {
			t.Fatalf("%s not accepted: op=%x err=%v", user, op, err)
		}
		return c
	}
	alice := open("alice", "pw-alice")
	writeUsers(t, e.usersFile,
		config.UserConfig{Username: "alice", Password: "pw-alice"},
		config.UserConfig{Username: "bob", Password: "pw-bob"})
	if _, err := e.srv.ReloadUsers(); err != nil {
		t.Fatal(err)
	}
	bob := open("bob", "pw-bob")

	// Remove alice: her open stream must end, bob's must keep working.
	writeUsers(t, e.usersFile, config.UserConfig{Username: "bob", Password: "pw-bob"})
	if _, err := e.srv.ReloadUsers(); err != nil {
		t.Fatal(err)
	}
	_ = alice.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := alice.readFrame(); err != nil {
			break // closed, as wanted
		}
	}
	bob.writeFrame(0x2, []byte("still here"))
	_ = bob.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	// The 'x' sent with the request header is echoed first.
	if got := readUntil(t, bob, nil, len("xstill here")); string(got) != "xstill here" {
		t.Fatalf("bob's stream disturbed: %q", got)
	}
}

func TestVLESSDestinationPolicy(t *testing.T) {
	e := startVLESS(t, false, false, nil) // production policy
	host, port := startEchoTCP(t)
	id := e.userID("alice", "pw-alice")
	for name, req := range map[string][]byte{
		"loopback IP":     buildVLESSRequest(id, vlessCmdTCP, host, port),
		"loopback name":   buildVLESSRequest(id, vlessCmdTCP, "localhost", port),
		"private IP":      buildVLESSRequest(id, vlessCmdTCP, "10.1.2.3", 80),
		"blocked port 25": buildVLESSRequest(id, vlessCmdTCP, "1.1.1.1", 25),
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := dialWS(t, e, testWSPath, "")
			defer c.conn.Close()
			c.writeFrame(0x2, req)
			op, _, err := c.readFrame()
			if err == nil && op != 0x8 {
				t.Fatalf("destination allowed (opcode %x)", op)
			}
		})
	}
}

func TestIsPublicIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"1.1.1.1": true, "8.8.8.8": true, "2606:4700::1111": true,
		"127.0.0.1": false, "10.0.0.1": false, "172.16.5.4": false, "192.168.1.1": false,
		"169.254.169.254": false, "100.64.0.1": false, "0.0.0.0": false, "::1": false,
		"fd00::1": false, "fe80::1": false, "::ffff:127.0.0.1": false, "224.0.0.1": false,
		"255.255.255.255": false,
	} {
		if got := isPublicIP(net.ParseIP(ip)); got != want {
			t.Errorf("isPublicIP(%s) = %v, want %v", ip, got, want)
		}
	}
}

func TestVLESSUDP(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, a, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			pc.WriteTo(append([]byte("re:"), buf[:n]...), a)
		}
	}()
	ua := pc.LocalAddr().(*net.UDPAddr)

	c, _ := dialWS(t, e, testWSPath, "")
	pkt := []byte("dns-ish query")
	req := buildVLESSRequest(e.userID("alice", "pw-alice"), vlessCmdUDP, ua.IP.String(), uint16(ua.Port))
	req = binary.BigEndian.AppendUint16(req, uint16(len(pkt)))
	req = append(req, pkt...)
	c.writeFrame(0x2, req)
	got := readUntil(t, c, expectResponseHeader(t, c), 2)
	n := int(binary.BigEndian.Uint16(got[:2]))
	got = readUntil(t, c, got, 2+n)
	if string(got[2:2+n]) != "re:dns-ish query" {
		t.Fatalf("udp reply %q", got[2:])
	}
}

func TestVLESSEarlyData(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	host, port := startEchoTCP(t)
	early := append(buildVLESSRequest(e.userID("alice", "pw-alice"), vlessCmdTCP, host, port), "0rtt"...)
	proto := base64.RawURLEncoding.EncodeToString(early)
	c, status := dialWS(t, e, testWSPath, "Sec-WebSocket-Protocol: "+proto+"\r\n")
	if !strings.HasPrefix(status, "101") || !strings.HasSuffix(status, "|"+proto) {
		t.Fatalf("upgrade %s: protocol not echoed", status)
	}
	if got := readUntil(t, c, expectResponseHeader(t, c), 4); string(got) != "0rtt" {
		t.Fatalf("early data echo %q", got)
	}
}

func TestVLESSDecoy(t *testing.T) {
	e := startVLESS(t, true, true, nil)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	for path, want := range map[string]int{"/": 200, "/admin": 404, testWSPath: 404} {
		resp, err := client.Get("https://" + e.addr + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != want || resp.Header.Get("Server") != "nginx" || !strings.Contains(string(body), "nginx") {
			t.Errorf("%s: status %d server %q", path, resp.StatusCode, resp.Header.Get("Server"))
		}
	}
	// Wrong path with a valid upgrade is a 404 too
	_, status := dialWS(t, e, "/other", "")
	if !strings.HasPrefix(status, "404") {
		t.Errorf("upgrade on wrong path: %s", status)
	}
}

func TestVLESSFrameTooLarge(t *testing.T) {
	e := startVLESS(t, false, true, nil)
	c, _ := dialWS(t, e, testWSPath, "")
	// Header claiming a 2 GiB payload must not be honored
	c.conn.Write([]byte{0x82, 0xFF, 0, 0, 0, 0, 0x80, 0, 0, 0, 1, 2, 3, 4})
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.br.ReadByte(); err == nil {
		// any bytes are fine as long as the connection then closes
		if _, err := io.ReadAll(c.br); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("connection not closed: %v", err)
		}
	}
}

// TestVLESSHandshakeDeadline: dripping the header in tiny frames doesn't
// keep a connection past the handshake timeout.
func TestVLESSHandshakeDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the handshake timeout")
	}
	e := startVLESS(t, false, true, nil)
	c, _ := dialWS(t, e, testWSPath, "")
	c.conn.SetDeadline(time.Now().Add(vlessHandshakeTimeout + 10*time.Second))
	start := time.Now()
	go func() {
		for i := 0; i < 17; i++ { // never completes the 18-byte header
			if c.writeFrame(0x2, []byte{0}) != nil {
				return
			}
			time.Sleep(time.Second)
		}
	}()
	for {
		if _, _, err := c.readFrame(); err != nil {
			break
		}
	}
	if d := time.Since(start); d > vlessHandshakeTimeout+5*time.Second {
		t.Fatalf("connection held for %v", d)
	}
}

func TestParsePortList(t *testing.T) {
	if p, err := ParsePortList("25, 465"); err != nil || len(p) != 2 || p[1] != 465 {
		t.Fatalf("got %v %v", p, err)
	}
	if p, err := ParsePortList("none"); err != nil || p != nil {
		t.Fatalf("none: %v %v", p, err)
	}
	if _, err := ParsePortList("0"); err == nil {
		t.Fatal("port 0 accepted")
	}
}
