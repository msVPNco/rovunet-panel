package proxy

import (
	"io"
	"net"
	"testing"
	"time"
)

// These tests cover the two guarantees required before the SOCKS5
// zero-user case can be called fixed: (1) a strict server with no
// configured users rejects every new connection attempt instead of
// falling back to an open relay, and (2) a strict server that reaches
// zero users while already running also drops sessions that had already
// authenticated, not just new login attempts.

func TestStrictServerRejectsNewConnWhenCredsEmpty(t *testing.T) {
	srv := NewServerMultiStrict("ignored", nil)

	client, serverSide := net.Pipe()
	done := make(chan struct{})
	go func() {
		srv.handleConn(serverSide)
		close(done)
	}()
	client.SetDeadline(time.Now().Add(2 * time.Second))

	// Greeting: client offers no-auth (0x00); a strict server with zero
	// users must demand auth (0x02) anyway, never fall back to no-auth.
	if _, err := client.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(client, methodReply); err != nil {
		t.Fatalf("read method reply: %v", err)
	}
	if methodReply[0] != 0x05 || methodReply[1] != 0x02 {
		t.Fatalf("expected server to demand auth (0x02) despite zero configured users, got %v", methodReply)
	}

	// Any username/password must fail — there are no users to match.
	user, pass := "anyone", "whatever"
	buf := append([]byte{0x01, byte(len(user))}, user...)
	buf = append(buf, byte(len(pass)))
	buf = append(buf, pass...)
	if _, err := client.Write(buf); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	authReply := make([]byte, 2)
	if _, err := io.ReadFull(client, authReply); err != nil {
		t.Fatalf("read auth reply: %v", err)
	}
	if authReply[1] != 0x01 {
		t.Fatalf("expected auth failure (0x01) with zero configured users, got %v", authReply)
	}

	client.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConn did not return after rejecting the connection")
	}
}

func TestNonStrictServerKeepsOpenByDesignWhenCredsEmpty(t *testing.T) {
	// Regression guard: loopback/intentionally-open listeners (created via
	// NewServerMulti, not NewServerMultiStrict) must keep their existing
	// no-auth behavior when no users are configured.
	srv := NewServerMulti("ignored", nil)

	client, serverSide := net.Pipe()
	go srv.handleConn(serverSide)
	client.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := client.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(client, methodReply); err != nil {
		t.Fatalf("read method reply: %v", err)
	}
	if methodReply[1] != 0x00 {
		t.Fatalf("expected no-auth method (0x00) for a non-strict server with no users, got %v", methodReply)
	}
	client.Close()
}

func TestStrictServerDropsLiveSessionWhenCredsEmptied(t *testing.T) {
	srv := NewServerMultiStrict("ignored", map[string]string{"u": "p"})

	client, serverSide := net.Pipe()
	done := make(chan struct{})
	go func() {
		srv.handleConn(serverSide)
		close(done)
	}()
	client.SetDeadline(time.Now().Add(2 * time.Second))

	// Authenticate successfully while a user still exists.
	if _, err := client.Write([]byte{0x05, 0x01, 0x02}); err != nil {
		t.Fatalf("write greeting: %v", err)
	}
	methodReply := make([]byte, 2)
	if _, err := io.ReadFull(client, methodReply); err != nil {
		t.Fatalf("read method reply: %v", err)
	}
	if methodReply[1] != 0x02 {
		t.Fatalf("expected server to request auth, got %v", methodReply)
	}
	if _, err := client.Write([]byte{0x01, 1, 'u', 1, 'p'}); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	authReply := make([]byte, 2)
	if _, err := io.ReadFull(client, authReply); err != nil {
		t.Fatalf("read auth reply: %v", err)
	}
	if authReply[1] != 0x00 {
		t.Fatalf("expected auth success while the user still exists, got %v", authReply)
	}

	// The session is now authenticated and blocked reading the CONNECT
	// request (a live, in-progress session). Remove the last user.
	srv.SetCredentials(nil)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("already-authenticated session was not dropped when the credential set was emptied")
	}
}
