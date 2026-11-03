package proxy

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func clientFrame(op byte, p []byte) []byte {
	var mask [4]byte
	rand.Read(mask[:])
	var h []byte
	switch {
	case len(p) <= 125:
		h = []byte{0x80 | op, 0x80 | byte(len(p))}
	case len(p) <= 65535:
		h = []byte{0x80 | op, 0x80 | 126, 0, 0}
		binary.BigEndian.PutUint16(h[2:], uint16(len(p)))
	}
	h = append(h, mask[:]...)
	m := make([]byte, len(p))
	for i := range p {
		m[i] = p[i] ^ mask[i%4]
	}
	return append(h, m...)
}

// TestStunTLSWebSocketPingGoesToClient: a ping mid-session must be answered
// to the client and must never leak into the SSH byte stream.
func TestStunTLSWebSocketPingGoesToClient(t *testing.T) {
	// Fake SSH backend that echoes everything.
	echo, _ := net.Listen("tcp", "127.0.0.1:0")
	defer echo.Close()
	got := make(chan []byte, 1)
	go func() {
		c, err := echo.Accept()
		if err != nil {
			return
		}
		var all bytes.Buffer
		buf := make([]byte, 4096)
		for {
			n, err := c.Read(buf)
			all.Write(buf[:n])
			c.Write(buf[:n])
			if err != nil || all.Len() >= 10 {
				got <- all.Bytes()
				c.Close()
				return
			}
		}
	}()

	s := &StunTLSServer{sshAddr: echo.Addr().String()}
	cli, srv := net.Pipe()
	go s.handleConn(srv)
	defer cli.Close()
	cli.SetDeadline(time.Now().Add(5 * time.Second))

	go cli.Write([]byte("GET /x HTTP/1.1\r\nHost: a\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"))
	br := bufio.NewReader(cli)
	line, _ := br.ReadString('\n')
	if !strings.Contains(line, "101") {
		t.Fatalf("no upgrade: %q", line)
	}
	for {
		l, _ := br.ReadString('\n')
		if l == "\r\n" {
			break
		}
	}
	go func() {
		cli.Write(clientFrame(0x2, []byte("hello")))
		cli.Write(clientFrame(0x9, []byte("pp")))
		cli.Write(clientFrame(0x2, []byte("world")))
	}()

	sawPong, data := false, ""
	for !(sawPong && len(data) >= 10) {
		op, p, err := readWSFrame(br)
		if err != nil {
			t.Fatalf("read: %v (pong=%v data=%q)", err, sawPong, data)
		}
		switch op {
		case 0xA:
			if string(p) != "pp" {
				t.Fatalf("pong payload %q", p)
			}
			sawPong = true
		case 0x2:
			data += string(p)
		}
	}
	if data != "helloworld" {
		t.Fatalf("echo %q", data)
	}
	select {
	case b := <-got:
		if string(b) != "helloworld" {
			t.Fatalf("backend saw %q (pong leaked into SSH stream?)", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("backend timeout")
	}
	_ = io.EOF
}
