package proxy

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReview10ShutdownCancelsDestinationDial(t *testing.T) {
	entered := make(chan struct{})
	environment := startVLESS(t, false, true, func(options *VLESSOptions) {
		options.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}
	})
	client, status := dialWS(t, environment, testWSPath, "")
	if !strings.HasPrefix(status, "101") {
		t.Fatal(status)
	}
	defer client.conn.Close()
	if err := client.writeFrame(2, buildVLESSRequest(environment.userID("alice", "pw-alice"), vlessCmdTCP, "127.0.0.1", 19090)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("destination dial was not entered")
	}
	if err := environment.srv.httpSrv.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-environment.done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel destination dial")
	}
}

func TestReview10ShutdownBarriers(t *testing.T) {
	for _, point := range []string{"before-hijack", "after-hijack", "before-handshake", "before-register", "after-register"} {
		t.Run(point, func(t *testing.T) {
			reached := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			environment := startVLESS(t, false, true, func(options *VLESSOptions) {
				options.testStage = func(stage string) {
					if stage == point {
						once.Do(func() { close(reached); <-release })
					}
				}
			})
			host, port := startEchoTCP(t)
			conn, err := net.Dial("tcp", environment.addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_, err = fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: localhost\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", testWSPath)
			if err != nil {
				t.Fatal(err)
			}
			clientDone := make(chan struct{})
			go func() {
				defer close(clientDone)
				reader := bufio.NewReader(conn)
				response, err := http.ReadResponse(reader, nil)
				if err != nil || response.StatusCode != 101 {
					return
				}
				client := &testWSClient{conn: conn, br: reader}
				request := append(buildVLESSRequest(environment.userID("alice", "pw-alice"), vlessCmdTCP, host, port), 'x')
				if err := client.writeFrame(2, request); err != nil {
					return
				}
				for {
					if _, _, err := client.readFrame(); err != nil {
						return
					}
				}
			}()
			select {
			case <-reached:
			case <-time.After(2 * time.Second):
				t.Fatal("barrier not reached")
			}
			if err := environment.srv.httpSrv.Close(); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for {
				environment.srv.sessMu.Lock()
				closing := environment.srv.closing
				environment.srv.sessMu.Unlock()
				if closing {
					break
				}
				if time.Now().After(deadline) {
					close(release)
					t.Fatal("shutdown did not begin")
				}
				time.Sleep(time.Millisecond)
			}
			close(release)
			select {
			case <-environment.done:
			case <-time.After(time.Second):
				conn.Close()
				t.Fatal("late or partial session prevented bounded shutdown")
			}
			conn.Close()
			select {
			case <-clientDone:
			case <-time.After(time.Second):
				t.Fatal("client did not exit")
			}
		})
	}
}

func TestReview10ShutdownPartialVLESSHeader(t *testing.T) {
	environment := startVLESS(t, false, true, nil)
	client, status := dialWS(t, environment, testWSPath, "")
	if !strings.HasPrefix(status, "101") {
		t.Fatal(status)
	}
	defer client.conn.Close()
	if err := client.writeFrame(2, []byte{0, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := environment.srv.httpSrv.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-environment.done:
	case <-time.After(time.Second):
		t.Fatal("partially authenticated hijacked connection survived shutdown")
	}
}
