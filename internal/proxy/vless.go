package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/keys"
)

// VLESS over WebSocket server.
//
// Wire format (what SlipNet's VlessBridge, Xray and sing-box send):
//
//	request : version(1)=0 | uuid(16) | addons_len(1) | addons | command(1) |
//	          port(2, big endian) | addr_type(1) | addr | payload...
//	response: version(1)=0 | addons_len(1)=0 | payload...
//
// command 1 is a TCP stream; command 2 is UDP to one destination, each packet
// framed as length(2, big endian) | data in both directions. Mux (command 3,
// used by XUDP) is not supported.
//
// Anything that isn't a WebSocket upgrade on the configured path gets the
// pages of a stock nginx install, so a scanner sees an ordinary web server.

const (
	vlessVersion = 0

	vlessCmdTCP = 1
	vlessCmdUDP = 2
	vlessCmdMux = 3

	vlessAddrIPv4   = 1
	vlessAddrDomain = 2
	vlessAddrIPv6   = 3

	// wsMaxFramePayload bounds one WebSocket frame. Clients send at most a
	// few tens of KB per frame; the cap stops a peer from making us
	// allocate an arbitrary amount.
	wsMaxFramePayload = 1 << 20

	vlessHandshakeTimeout = 20 * time.Second // upgrade → VLESS header
	vlessDialTimeout      = 10 * time.Second
	vlessPingInterval     = 30 * time.Second // keeps CDN idle timers (~100 s) from closing quiet streams
	vlessReadTimeout      = 3 * vlessPingInterval
	vlessWriteTimeout     = 30 * time.Second
	vlessUDPIdleTimeout   = 3 * time.Minute
	vlessUsersPoll        = 5 * time.Second
	vlessMaxEarlyData     = 8 << 10
)

var (
	errVLESSAuth        = errors.New("vless: unknown user id")
	errVLESSDestination = errors.New("vless: destination not allowed")
)

// VLESSOptions configures a VLESS server.
type VLESSOptions struct {
	Listen     string // host:port to listen on
	Path       string // WebSocket path, e.g. "/a1b2c3"
	CertFile   string // TLS certificate; empty serves plain HTTP (e.g. behind a CDN on port 80)
	KeyFile    string
	SecretFile string // secret that turns user credentials into VLESS UUIDs
	UsersFile  string // rovunet config.json holding the users
	BlockPorts []int  // destination ports refused for every user

	// AllowPrivate lets streams reach loopback, private and other
	// non-public addresses. Off in production: those would expose
	// services on the server and its network to every user.
	AllowPrivate bool
	Verbose      bool

	// dial reaches destinations; tests override it.
	dial func(ctx context.Context, network, address string) (net.Conn, error)
	// lookup resolves destination names; tests override it.
	lookup func(ctx context.Context, host string) ([]net.IP, error)
}

// VLESSServer serves VLESS over WebSocket.
type VLESSServer struct {
	opts      VLESSOptions
	secret    []byte
	tlsConfig *tls.Config
	blocked   map[uint16]bool

	users     atomic.Pointer[map[[16]byte]string] // id → username
	usersStat atomic.Value                        // fileStamp of the last load

	httpSrv *http.Server

	sessMu   sync.Mutex
	sessions map[*wsConn][16]byte // live sessions → the id they authenticated with
}

type fileStamp struct {
	mod  time.Time
	size int64
}

// NewVLESSServer loads the secret, certificate and users.
func NewVLESSServer(opts VLESSOptions) (*VLESSServer, error) {
	if !strings.HasPrefix(opts.Path, "/") {
		return nil, fmt.Errorf("vless: path must start with '/'")
	}
	secret, err := keys.LoadVLESSSecret(opts.SecretFile)
	if err != nil {
		return nil, fmt.Errorf("vless: %w", err)
	}

	s := &VLESSServer{opts: opts, secret: secret, blocked: make(map[uint16]bool)}
	for _, p := range opts.BlockPorts {
		if p > 0 && p < 65536 {
			s.blocked[uint16(p)] = true
		}
	}
	if s.opts.dial == nil {
		d := &net.Dialer{Timeout: vlessDialTimeout, KeepAlive: 30 * time.Second}
		s.opts.dial = d.DialContext
	}
	if s.opts.lookup == nil {
		s.opts.lookup = func(ctx context.Context, host string) ([]net.IP, error) {
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			ips := make([]net.IP, len(addrs))
			for i, a := range addrs {
				ips[i] = a.IP
			}
			return ips, nil
		}
	}

	if opts.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("vless: load TLS cert: %w", err)
		}
		s.tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			// WebSocket upgrades need HTTP/1.1; a CDN or client offering h2
			// falls back to it.
			NextProtos: []string{"http/1.1"},
		}
	}

	if _, err := s.ReloadUsers(); err != nil {
		return nil, err
	}
	return s, nil
}

// ReloadUsers re-reads the users file and swaps the set of valid IDs.
// Streams already open keep running.
func (s *VLESSServer) ReloadUsers() (int, error) {
	st, err := os.Stat(s.opts.UsersFile)
	if err != nil {
		return 0, fmt.Errorf("vless: users file: %w", err)
	}
	cfg, err := config.LoadFrom(s.opts.UsersFile)
	if err != nil {
		return 0, fmt.Errorf("vless: users file: %w", err)
	}
	users := make(map[[16]byte]string, len(cfg.Users))
	for _, u := range cfg.Users {
		users[keys.VLESSUserID(s.secret, u.Username, u.Password)] = u.Username
	}
	s.users.Store(&users)
	s.usersStat.Store(fileStamp{st.ModTime(), st.Size()})
	s.dropRevoked(users)
	return len(users), nil
}

// watchUsers reloads the users whenever the users file changes, so adding,
// removing or re-keying a user takes effect within a few seconds without
// touching open streams.
func (s *VLESSServer) watchUsers(ctx context.Context) {
	t := time.NewTicker(vlessUsersPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		st, err := os.Stat(s.opts.UsersFile)
		if err != nil {
			continue
		}
		if last, ok := s.usersStat.Load().(fileStamp); ok && last.mod.Equal(st.ModTime()) && last.size == st.Size() {
			continue
		}
		if n, err := s.ReloadUsers(); err != nil {
			log.Printf("%v", err)
		} else {
			log.Printf("vless: users reloaded (%d)", n)
		}
	}
}

// dropRevoked closes every live session whose id is no longer valid, so a
// removed or re-keyed user is cut off at once and not only at the next
// connection. Sessions of other users are left alone.
func (s *VLESSServer) dropRevoked(valid map[[16]byte]string) {
	s.sessMu.Lock()
	var gone []*wsConn
	for ws, id := range s.sessions {
		if _, ok := valid[id]; !ok {
			gone = append(gone, ws)
		}
	}
	s.sessMu.Unlock()
	for _, ws := range gone {
		_ = ws.conn.Close()
	}
	if len(gone) > 0 {
		log.Printf("vless: closed %d session(s) of removed users", len(gone))
	}
}

func (s *VLESSServer) trackSession(ws *wsConn, id [16]byte, add bool) {
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	if !add {
		delete(s.sessions, ws)
		return
	}
	if s.sessions == nil {
		s.sessions = make(map[*wsConn][16]byte)
	}
	s.sessions[ws] = id
}

func (s *VLESSServer) lookupUser(id [16]byte) (string, bool) {
	users := s.users.Load()
	if users == nil {
		return "", false
	}
	name, ok := (*users)[id]
	return name, ok
}

func (s *VLESSServer) logf(format string, args ...any) {
	if s.opts.Verbose {
		log.Printf(format, args...)
	}
}

// ListenAndServe listens on opts.Listen and serves until ctx ends.
func (s *VLESSServer) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.opts.Listen)
	if err != nil {
		return fmt.Errorf("vless: listen: %w", err)
	}
	return s.Serve(ctx, ln)
}

// Serve serves on ln until ctx ends.
func (s *VLESSServer) Serve(ctx context.Context, ln net.Listener) error {
	if s.tlsConfig != nil {
		ln = tls.NewListener(ln, s.tlsConfig)
	}
	s.httpSrv = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    16 << 10,
		// Scanners and probes produce endless TLS/HTTP errors; not news.
		ErrorLog:     log.New(io.Discard, "", 0),
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
	}

	go s.watchUsers(ctx)
	go func() {
		<-ctx.Done()
		_ = s.httpSrv.Close()
	}()

	scheme := "http"
	if s.tlsConfig != nil {
		scheme = "https"
	}
	log.Printf("vless: listening on %s (%s, path %s)", ln.Addr(), scheme, s.opts.Path)
	err := s.httpSrv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// ServeHTTP upgrades WebSocket requests on the VLESS path and answers
// everything else like a stock nginx.
func (s *VLESSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.opts.Path || r.Method != http.MethodGet || !isWebSocketUpgrade(r) {
		serveDecoy(w, r)
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" || r.Header.Get("Sec-WebSocket-Version") != "13" {
		serveDecoy(w, r)
		return
	}

	// Xray-style 0-RTT: the first bytes ride base64url-encoded in
	// Sec-WebSocket-Protocol, which the response must echo.
	var early []byte
	protocol := r.Header.Get("Sec-WebSocket-Protocol")
	if protocol != "" {
		b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(protocol, "="))
		if err != nil || len(b) == 0 || len(b) > vlessMaxEarlyData {
			protocol = ""
		} else {
			early = b
		}
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		serveDecoy(w, r)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Time{})

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + computeAcceptKey(key) + "\r\n"
	if protocol != "" {
		resp += "Sec-WebSocket-Protocol: " + protocol + "\r\n"
	}
	resp += "\r\n"
	_ = conn.SetWriteDeadline(time.Now().Add(vlessWriteTimeout))
	if _, err := conn.Write([]byte(resp)); err != nil {
		return
	}

	ws := newWSConn(conn, rw.Reader)
	s.handleSession(ws, early, r.RemoteAddr)
}

func isWebSocketUpgrade(r *http.Request) bool {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	for _, v := range r.Header.Values("Connection") {
		for _, tok := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), "upgrade") {
				return true
			}
		}
	}
	return false
}

type vlessRequest struct {
	id      [16]byte
	user    string
	command byte
	host    string // IP literal or domain name
	port    uint16
}

func (req *vlessRequest) target() string {
	return net.JoinHostPort(req.host, strconv.Itoa(int(req.port)))
}

// readVLESSRequest reads and authenticates a request header.
func (s *VLESSServer) readVLESSRequest(r io.Reader) (*vlessRequest, error) {
	var hdr [18]byte // version | uuid | addons_len
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	if hdr[0] != vlessVersion {
		return nil, fmt.Errorf("vless: unsupported version %d", hdr[0])
	}
	var id [16]byte
	copy(id[:], hdr[1:17])
	user, ok := s.lookupUser(id)
	if !ok {
		return nil, errVLESSAuth
	}
	// Addons (e.g. a flow setting) carry nothing we act on over WebSocket.
	if n := int64(hdr[17]); n > 0 {
		if _, err := io.CopyN(io.Discard, r, n); err != nil {
			return nil, err
		}
	}

	var cmd [1]byte
	if _, err := io.ReadFull(r, cmd[:]); err != nil {
		return nil, err
	}
	req := &vlessRequest{id: id, user: user, command: cmd[0]}
	switch req.command {
	case vlessCmdTCP, vlessCmdUDP:
	case vlessCmdMux:
		return req, fmt.Errorf("vless: mux (XUDP) is not supported; turn mux off in the client")
	default:
		return req, fmt.Errorf("vless: unknown command %d", req.command)
	}

	var pa [3]byte // port | addr_type
	if _, err := io.ReadFull(r, pa[:]); err != nil {
		return nil, err
	}
	req.port = binary.BigEndian.Uint16(pa[:2])
	switch pa[2] {
	case vlessAddrIPv4:
		var ip [4]byte
		if _, err := io.ReadFull(r, ip[:]); err != nil {
			return nil, err
		}
		req.host = net.IP(ip[:]).String()
	case vlessAddrIPv6:
		var ip [16]byte
		if _, err := io.ReadFull(r, ip[:]); err != nil {
			return nil, err
		}
		req.host = net.IP(ip[:]).String()
	case vlessAddrDomain:
		var n [1]byte
		if _, err := io.ReadFull(r, n[:]); err != nil {
			return nil, err
		}
		name := make([]byte, n[0])
		if _, err := io.ReadFull(r, name); err != nil {
			return nil, err
		}
		req.host = string(name)
		if req.host == "" {
			return req, fmt.Errorf("vless: empty domain")
		}
	default:
		return req, fmt.Errorf("vless: unknown address type %d", pa[2])
	}
	if req.port == 0 {
		return req, fmt.Errorf("vless: port 0")
	}
	return req, nil
}

func (s *VLESSServer) handleSession(ws *wsConn, early []byte, remote string) {
	var r io.Reader = ws
	if len(early) > 0 {
		r = io.MultiReader(bytes.NewReader(early), ws)
	}

	// The whole header must arrive in time, however it is split into
	// frames, so a slow client can't hold the connection open.
	ws.readTimeout = vlessHandshakeTimeout
	handshakeTimer := time.AfterFunc(vlessHandshakeTimeout, func() { _ = ws.conn.Close() })
	req, err := s.readVLESSRequest(r)
	if !handshakeTimer.Stop() && err == nil {
		err = errors.New("vless: handshake timed out")
	}
	if err != nil {
		if errors.Is(err, errVLESSAuth) {
			s.logf("vless: %s: rejected unknown user id", remote)
		} else {
			s.logf("vless: %s: bad request: %v", remote, err)
		}
		ws.writeClose(1008) // policy violation
		return
	}
	ws.readTimeout = vlessReadTimeout

	// Register before checking again, so a reload between the two cannot
	// leave a removed user's session unnoticed.
	s.trackSession(ws, req.id, true)
	defer s.trackSession(ws, req.id, false)
	if _, ok := s.lookupUser(req.id); !ok {
		ws.writeClose(1008)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), vlessDialTimeout)
	network := "tcp"
	if req.command == vlessCmdUDP {
		network = "udp"
	}
	up, err := s.dialDestination(ctx, network, req.host, req.port)
	cancel()
	if err != nil {
		s.logf("vless: %s (%s): %s %s: %v", remote, req.user, network, req.target(), err)
		ws.writeClose(1011)
		return
	}
	defer up.Close()
	s.logf("vless: %s (%s): %s %s", remote, req.user, network, req.target())

	// Respond right away: with server-first protocols such as SSH the
	// client sends nothing until it has the header.
	if err := ws.writeMessage([]byte{vlessVersion, 0}); err != nil {
		return
	}

	stop := make(chan struct{})
	defer close(stop)
	go ws.keepAlive(stop)

	// Traffic is counted per user in both directions (see Meter).
	r = &countReader{r: r, user: req.user}
	if req.command == vlessCmdUDP {
		relayVLESSUDP(ws, r, up, req.user)
	} else {
		relayVLESSTCP(ws, r, up, req.user)
	}
}

// relayVLESSTCP copies both ways until either side finishes, then tears
// both down: a WebSocket close ends the stream in both directions.
func relayVLESSTCP(ws *wsConn, client io.Reader, up net.Conn, user string) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(up, client)
		done <- struct{}{}
	}()
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := up.Read(buf)
			if n > 0 {
				if werr := ws.writeMessage(buf[:n]); werr != nil {
					break
				}
				meterAdd(user, int64(n))
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()
	<-done
	ws.writeClose(1000)
	_ = up.Close()
	_ = ws.conn.Close()
	<-done
}

// relayVLESSUDP carries length-prefixed packets to and from one destination.
func relayVLESSUDP(ws *wsConn, client io.Reader, up net.Conn, user string) {
	done := make(chan struct{}, 2)
	go func() {
		var lenBuf [2]byte
		pkt := make([]byte, 65535)
		for {
			if _, err := io.ReadFull(client, lenBuf[:]); err != nil {
				break
			}
			n := int(binary.BigEndian.Uint16(lenBuf[:]))
			if _, err := io.ReadFull(client, pkt[:n]); err != nil {
				break
			}
			if _, err := up.Write(pkt[:n]); err != nil {
				break
			}
		}
		done <- struct{}{}
	}()
	go func() {
		buf := make([]byte, 2+65535)
		for {
			_ = up.SetReadDeadline(time.Now().Add(vlessUDPIdleTimeout))
			n, err := up.Read(buf[2:])
			if err != nil {
				break
			}
			binary.BigEndian.PutUint16(buf[:2], uint16(n))
			if err := ws.writeMessage(buf[:2+n]); err != nil {
				break
			}
			meterAdd(user, int64(n))
		}
		done <- struct{}{}
	}()
	<-done
	ws.writeClose(1000)
	_ = up.Close()
	_ = ws.conn.Close()
	<-done
}

// dialDestination applies the destination policy, resolving names itself so
// the checked address is the one dialed.
func (s *VLESSServer) dialDestination(ctx context.Context, network, host string, port uint16) (net.Conn, error) {
	if s.blocked[port] {
		return nil, fmt.Errorf("%w: port %d is blocked", errVLESSDestination, port)
	}

	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		var err error
		ips, err = s.opts.lookup(ctx, host)
		if err != nil {
			return nil, err
		}
		// IPv4 first: many servers have no working IPv6 route
		sort.SliceStable(ips, func(i, j int) bool {
			return ips[i].To4() != nil && ips[j].To4() == nil
		})
	}

	var lastErr error
	for _, ip := range ips {
		if !s.opts.AllowPrivate && !isPublicIP(ip) {
			if lastErr == nil {
				lastErr = fmt.Errorf("%w: %s is not a public address", errVLESSDestination, ip)
			}
			continue
		}
		conn, err := s.opts.dial(ctx, network, net.JoinHostPort(ip.String(), strconv.Itoa(int(port))))
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no addresses for %s", host)
	}
	return nil, lastErr
}

// nonPublicNets lists special-purpose ranges the net.IP predicates miss.
var nonPublicNets = func() []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range []string{
		"0.0.0.0/8",       // "this network"
		"100.64.0.0/10",   // carrier-grade NAT
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // documentation
		"198.18.0.0/15",   // benchmarking
		"198.51.100.0/24", // documentation
		"203.0.113.0/24",  // documentation
		"240.0.0.0/4",     // reserved, broadcast
		"100::/64",        // discard
		"2001:db8::/32",   // documentation
	} {
		_, n, _ := net.ParseCIDR(cidr)
		nets = append(nets, n)
	}
	return nets
}()

// isPublicIP reports whether ip is a globally routable unicast address.
func isPublicIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	for _, n := range nonPublicNets {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

// ParsePortList parses "25,465" (or "none") into ports.
func ParsePortList(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return nil, nil
	}
	var ports []int
	for _, f := range strings.Split(s, ",") {
		p, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid port %q", f)
		}
		ports = append(ports, p)
	}
	return ports, nil
}

// --- WebSocket (RFC 6455), server side ---

// wsConn reads client frames as a byte stream and writes server frames.
// Control frames are handled inline: pings get pongs, close ends the stream.
type wsConn struct {
	conn        net.Conn
	br          *bufio.Reader
	readTimeout time.Duration

	wmu    sync.Mutex
	closed bool // close frame sent

	pending []byte // unread part of the current data frame
}

func newWSConn(conn net.Conn, br *bufio.Reader) *wsConn {
	return &wsConn{conn: conn, br: br, readTimeout: vlessReadTimeout}
}

func (c *wsConn) Read(p []byte) (int, error) {
	for len(c.pending) == 0 {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return 0, err
		}
		switch opcode {
		case 0x0, 0x1, 0x2: // continuation, text, binary
			c.pending = payload
		case 0x8: // close
			code := uint16(1000)
			if len(payload) >= 2 {
				code = binary.BigEndian.Uint16(payload[:2])
			}
			c.writeClose(code)
			return 0, io.EOF
		case 0x9: // ping
			if err := c.writeFrame(0xA, payload); err != nil {
				return 0, err
			}
		case 0xA: // pong
		default:
			return 0, fmt.Errorf("websocket: unknown opcode %#x", opcode)
		}
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

func (c *wsConn) readFrame() (opcode byte, payload []byte, err error) {
	if c.readTimeout > 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}
	var hdr [2]byte
	if _, err = io.ReadFull(c.br, hdr[:]); err != nil {
		return
	}
	if hdr[0]&0x70 != 0 {
		return 0, nil, errors.New("websocket: reserved bits set")
	}
	opcode = hdr[0] & 0x0F
	masked := hdr[1]&0x80 != 0
	length := uint64(hdr[1] & 0x7F)
	switch length {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return
		}
		length = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.br, ext[:]); err != nil {
			return
		}
		length = binary.BigEndian.Uint64(ext[:])
	}
	if opcode >= 0x8 && (length > 125 || hdr[0]&0x80 == 0) {
		return 0, nil, errors.New("websocket: invalid control frame")
	}
	if length > wsMaxFramePayload {
		return 0, nil, fmt.Errorf("websocket: frame too large (%d bytes)", length)
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(c.br, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, length)
	if _, err = io.ReadFull(c.br, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i&3]
		}
	}
	return opcode, payload, nil
}

// writeFrame sends one unmasked frame in a single write.
func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	return c.writeFrameLocked(opcode, payload)
}

func (c *wsConn) writeFrameLocked(opcode byte, payload []byte) error {
	n := len(payload)
	frame := make([]byte, 0, 10+n)
	frame = append(frame, 0x80|opcode)
	switch {
	case n <= 125:
		frame = append(frame, byte(n))
	case n <= 0xFFFF:
		frame = append(frame, 126, byte(n>>8), byte(n))
	default:
		frame = append(frame, 127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(n))
	}
	frame = append(frame, payload...)
	_ = c.conn.SetWriteDeadline(time.Now().Add(vlessWriteTimeout))
	_, err := c.conn.Write(frame)
	return err
}

func (c *wsConn) writeMessage(p []byte) error { return c.writeFrame(0x2, p) }

// writeClose sends a close frame once; later writes fail.
func (c *wsConn) writeClose(code uint16) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed {
		return
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], code)
	_ = c.writeFrameLocked(0x8, p[:])
	c.closed = true
}

// keepAlive pings until stop closes. CDNs drop WebSockets that stay silent
// for about 100 s, and the client's pongs keep our read deadline fresh.
func (c *wsConn) keepAlive(stop <-chan struct{}) {
	t := time.NewTicker(vlessPingInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if err := c.writeFrame(0x9, nil); err != nil {
				return
			}
		}
	}
}

// --- decoy: a stock nginx install ---

func serveDecoy(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Server", "nginx")
	h.Set("Content-Type", "text/html")
	if r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, nginxWelcomePage)
		}
		return
	}
	w.WriteHeader(http.StatusNotFound)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, nginxNotFoundPage)
	}
}

const nginxWelcomePage = `<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
`

const nginxNotFoundPage = `<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>nginx</center>
</body>
</html>
`
