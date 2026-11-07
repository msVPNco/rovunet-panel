package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Server is a SOCKS5 proxy server supporting CONNECT with optional auth.
// Credentials can be swapped at runtime via SetCredentials — used to
// propagate user add/remove events without restarting the listener and
// dropping live client connections.
type Server struct {
	listenAddr string

	// requireAuth marks a server where an empty credential set must mean
	// "accept no one", never "accept everyone". Loopback/intentionally-open
	// listeners (created via NewServer/NewServerMulti) leave this false, so
	// an empty map there keeps meaning no-auth, exactly as before. Listeners
	// reachable from outside the host should be created via
	// NewServerMultiStrict instead, which sets this true.
	requireAuth bool

	credsMu     sync.RWMutex
	credentials map[string]string // username → password (empty map = no auth, unless requireAuth)

	connsMu sync.Mutex
	conns   map[net.Conn]struct{}
	logins  map[net.Conn][2]string // live connection → the login it used
}

// NewServer creates a SOCKS5 server with a single credential pair.
// For multiple users, use NewServerMulti.
func NewServer(addr string, user, pass string) *Server {
	creds := make(map[string]string)
	if user != "" {
		creds[user] = pass
	}
	return &Server{listenAddr: addr, credentials: creds}
}

// NewServerMulti creates a SOCKS5 server with multiple credential pairs. An
// empty credential set means no auth is required — use this constructor
// only for listeners that are loopback-only or otherwise meant to be open
// by design. For a listener reachable from outside the host, use
// NewServerMultiStrict so a config change that empties the credential set
// cannot silently turn it into an open relay.
func NewServerMulti(addr string, creds map[string]string) *Server {
	if creds == nil {
		creds = make(map[string]string)
	}
	return &Server{listenAddr: addr, credentials: creds}
}

// NewServerMultiStrict is like NewServerMulti, but an empty credential set
// (now, or after a later SetCredentials call) rejects every connection
// instead of allowing unauthenticated access. A SetCredentials call that
// empties the set also closes every connection already open on this
// server, so reaching zero configured users leaves no live session
// running either. Use this for any listener bound to a non-loopback
// address.
func NewServerMultiStrict(addr string, creds map[string]string) *Server {
	s := NewServerMulti(addr, creds)
	s.requireAuth = true
	return s
}

// SetCredentials replaces the server's credential set atomically. New
// connections authenticate against the new set. For a server created with
// NewServerMulti, an empty map disables auth for new connections, same as
// before, and existing connections are left running. For a server created
// with NewServerMultiStrict, an empty map instead rejects every new
// connection and closes every connection already open.
func (s *Server) SetCredentials(creds map[string]string) {
	if creds == nil {
		creds = make(map[string]string)
	}
	s.credsMu.Lock()
	s.credentials = creds
	emptyAndStrict := s.requireAuth && len(creds) == 0
	s.credsMu.Unlock()

	if emptyAndStrict {
		s.closeAllConns()
		return
	}
	s.dropRevoked()
}

// dropRevoked closes the open connections whose login is no longer in the
// credential set (a user was removed or got a new password), so they end now
// and not only when the client reconnects. Other users are not touched.
func (s *Server) dropRevoked() {
	s.connsMu.Lock()
	var gone []net.Conn
	for c, l := range s.logins {
		if !s.checkCredential(l[0], l[1]) {
			gone = append(gone, c)
		}
	}
	s.connsMu.Unlock()
	for _, c := range gone {
		c.Close()
	}
}

// noteLogin remembers which login a connection used.
func (s *Server) noteLogin(conn net.Conn, user, pass string) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if s.logins == nil {
		s.logins = make(map[net.Conn][2]string)
	}
	s.logins[conn] = [2]string{user, pass}
}

// hasAuth reports whether auth is currently required.
func (s *Server) hasAuth() bool {
	s.credsMu.RLock()
	defer s.credsMu.RUnlock()
	return s.requireAuth || len(s.credentials) > 0
}

// trackConn records or forgets a live connection so closeAllConns can reach it.
func (s *Server) trackConn(conn net.Conn, add bool) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if s.conns == nil {
		s.conns = make(map[net.Conn]struct{})
	}
	if add {
		s.conns[conn] = struct{}{}
	} else {
		delete(s.conns, conn)
		delete(s.logins, conn)
	}
}

// closeAllConns forcibly closes every connection currently tracked for this
// server. Safe to call concurrently with handleConn's own close of the same
// connection: net.Conn.Close is idempotent-safe to call twice.
func (s *Server) closeAllConns() {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	for c := range s.conns {
		c.Close()
	}
}

// checkCredential reports whether (user, pass) is in the current cred set.
func (s *Server) checkCredential(user, pass string) bool {
	s.credsMu.RLock()
	defer s.credsMu.RUnlock()
	expected, ok := s.credentials[user]
	return ok && expected == pass
}

// ListenAndServe starts the SOCKS5 server (blocking).
func (s *Server) ListenAndServe() error {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}

	ln, err := lc.Listen(context.Background(), "tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	log.Printf("SOCKS5 proxy listening on %s", s.listenAddr)

	// Graceful shutdown on SIGTERM/SIGINT
	done := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		log.Println("shutting down SOCKS5 proxy")
		close(done)
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-done:
				return nil
			default:
			}
			log.Printf("accept: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	s.trackConn(conn, true)
	defer s.trackConn(conn, false)
	defer conn.Close()

	// Greeting: client sends version + auth methods
	buf := make([]byte, 258)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	if buf[0] != 0x05 {
		return // not SOCKS5
	}
	nMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return
	}

	if s.hasAuth() {
		// Require username/password auth (method 0x02)
		conn.Write([]byte{0x05, 0x02})
		if !s.authenticate(conn) {
			return
		}
	} else {
		// No auth (method 0x00)
		conn.Write([]byte{0x05, 0x00})
	}

	// Request: version(1) + cmd(1) + rsv(1) + atyp(1)
	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return
	}
	if buf[0] != 0x05 || buf[2] != 0x00 {
		return
	}
	cmd := buf[1]
	atyp := buf[3]

	if cmd != 0x01 {
		// Only CONNECT supported
		s.sendReply(conn, 0x07, nil) // command not supported
		return
	}

	// Parse destination address
	var dstAddr string
	switch atyp {
	case 0x01: // IPv4
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return
		}
		dstAddr = net.IP(buf[:4]).String()
	case 0x03: // Domain
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		domainLen := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:domainLen]); err != nil {
			return
		}
		dstAddr = string(buf[:domainLen])
	case 0x04: // IPv6
		if _, err := io.ReadFull(conn, buf[:16]); err != nil {
			return
		}
		dstAddr = net.IP(buf[:16]).String()
	default:
		s.sendReply(conn, 0x08, nil) // address type not supported
		return
	}

	// Port (2 bytes, big-endian)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	port := int(buf[0])<<8 | int(buf[1])
	target := net.JoinHostPort(dstAddr, fmt.Sprintf("%d", port))

	// Connect to target
	remote, err := net.DialTimeout("tcp", target, 30*time.Second)
	if err != nil {
		s.sendReply(conn, 0x05, nil) // connection refused
		return
	}
	defer remote.Close()

	// Success reply
	s.sendReply(conn, 0x00, remote.LocalAddr().(*net.TCPAddr))

	// Bidirectional relay
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(remote, conn)
		if tc, ok := remote.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		io.Copy(conn, remote)
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	wg.Wait()
}

func (s *Server) authenticate(conn net.Conn) bool {
	// RFC 1929: version(1) + ulen(1) + username(ulen) + plen(1) + password(plen)
	buf := make([]byte, 513)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return false
	}
	if buf[0] != 0x01 {
		conn.Write([]byte{0x01, 0x01}) // failure
		return false
	}
	ulen := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:ulen]); err != nil {
		return false
	}
	user := string(buf[:ulen])

	if _, err := io.ReadFull(conn, buf[:1]); err != nil {
		return false
	}
	plen := int(buf[0])
	if _, err := io.ReadFull(conn, buf[:plen]); err != nil {
		return false
	}
	pass := string(buf[:plen])

	if s.checkCredential(user, pass) {
		// Remember the login, then check again: a reload in between must
		// not leave a removed user's connection running.
		s.noteLogin(conn, user, pass)
		if !s.checkCredential(user, pass) {
			conn.Write([]byte{0x01, 0x01})
			return false
		}
		conn.Write([]byte{0x01, 0x00}) // success
		return true
	}
	conn.Write([]byte{0x01, 0x01}) // failure
	return false
}

func (s *Server) sendReply(conn net.Conn, status byte, addr *net.TCPAddr) {
	reply := []byte{0x05, status, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	if addr != nil {
		ip := addr.IP.To4()
		if ip != nil {
			copy(reply[4:8], ip)
		}
		reply[8] = byte(addr.Port >> 8)
		reply[9] = byte(addr.Port)
	}
	conn.Write(reply)
}

// Serve starts the built-in SOCKS5 proxy.
func Serve(addr string, port int, user, pass string) error {
	listenAddr := fmt.Sprintf("%s:%d", addr, port)
	srv := NewServer(listenAddr, user, pass)
	return srv.ListenAndServe()
}

// ServeMulti starts the built-in SOCKS5 proxy with multiple credentials.
func ServeMulti(addr string, port int, creds map[string]string) error {
	listenAddr := fmt.Sprintf("%s:%d", addr, port)
	srv := NewServerMulti(listenAddr, creds)
	return srv.ListenAndServe()
}
