package proxy

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"net"
	"strings"
	"testing"
	"time"
)

func TestServerShutdownClosesHijackedStreams(t *testing.T) {
	environment := startVLESS(t, false, true, nil)
	host, port := startEchoTCP(t)
	writeUsers(t, environment.usersFile,
		config.UserConfig{Username: "alice", Password: "pw-alice"},
		config.UserConfig{Username: "bob", Password: "pw-bob"})
	if _, err := environment.srv.ReloadUsers(); err != nil {
		t.Fatal(err)
	}
	var clients []*testWSClient
	for _, user := range []string{"alice", "bob"} {
		client, status := dialWS(t, environment, testWSPath, "")
		if !strings.HasPrefix(status, "101") {
			t.Fatal(status)
		}
		defer client.conn.Close()
		request := append(buildVLESSRequest(environment.userID(user, "pw-"+user), vlessCmdTCP, host, port), 'x')
		if err := client.writeFrame(2, request); err != nil {
			t.Fatal(err)
		}
		readUntil(t, client, expectResponseHeader(t, client), 1)
		clients = append(clients, client)
	}
	if err := environment.srv.httpSrv.Close(); err != nil {
		t.Fatal(err)
	}
	for _, client := range clients {
		client.conn.SetReadDeadline(time.Now().Add(time.Second))
		for {
			_, _, err := client.readFrame()
			if err == nil {
				continue
			}
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatal("shutdown left hijacked stream active")
			}
			break
		}
	}
}
