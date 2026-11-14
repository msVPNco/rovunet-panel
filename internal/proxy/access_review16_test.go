package proxy

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
)

func need16(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func signal16(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(3 * time.Second):
		t.Fatal("barrier timeout")
	}
}
func TestGateConfigProcess(t *testing.T) {
	path := os.Getenv("RV16_CONFIG")
	if path == "" {
		t.Skip("subprocess helper")
	}
	c, e := config.LoadFrom(path)
	need16(t, e)
	u := *c.GetUser("alice")
	switch os.Getenv("RV16_ACTION") {
	case "revoke":
		c.RemoveUser("alice")
	case "password":
		u.Password = "replacement"
		u.SubToken = config.NewSubToken()
		c.AddUser(u)
	case "renew":
		u.PeriodID = "p2"
		c.AddUser(u)
	default:
		t.Fatal("bad fixture")
	}
	need16(t, c.Save())
}

type gateNet16 struct {
	net.Conn
	transform func([]byte) (int, error)
}

func (g gateNet16) Write(p []byte) (int, error) { return g.transform(p) }
func TestFiniteInFlightGate(t *testing.T) {
	for _, action := range []string{"revoke", "password", "renew", "expiry", "shutdown", "short-in-flight", "settlement-failed", "settle-reply-lost", "reserve-reply-lost", "idle-at-limit"} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			dir := filepath.Join(t.TempDir(), "db")
			st, e := quota.Create(ctx, dir, "server", "store")
			need16(t, e)
			defer st.Close()
			alice := quota.Key{Account: "A", Period: "p1"}
			bob := quota.Key{Account: "B", Period: "p1"}
			for _, k := range []quota.Key{alice, bob} {
				need16(t, st.NewPeriod(ctx, k, 1<<30))
			}
			need16(t, st.FinishBootstrap(dir))
			reached, unblock := make(chan struct{}), make(chan struct{})
			var once sync.Once
			release := func() { once.Do(func() { close(unblock) }) }
			defer release()
			var written atomic.Int64
			host, bobPort := startEchoTCP(t)
			alicePort := uint16(1)
			if bobPort == 1 {
				alicePort = 2
			}
			// Alice has a distinct observed destination. A real socket pair also
			// lets a 13-byte partial write block until cancellation closes the socket.
			var used atomic.Bool
			cfg := &config.Config{ServerID: "server", Users: []config.UserConfig{
				{Username: "alice", Password: "pw-alice", AccountID: "A", PeriodID: "p1", QuotaGB: 1, SubToken: config.NewSubToken()},
				{Username: "bob", Password: "pw-bob", AccountID: "B", PeriodID: "p1", QuotaGB: 1},
			}}
			if action == "idle-at-limit" {
				need16(t, st.ImportOpening(ctx, quota.Checkpoint{Server: "server", Store: "store", Account: "A", Period: "p1", ID: "base", OpeningOrigin: "past", Proof: "disjoint synthetic opening", Opening: (1 << 30) - 100}))
			}
			if action == "expiry" {
				cfg.Users[0].ExpiresAt = time.Now().Add(1250 * time.Millisecond).UTC().Format(time.RFC3339Nano)
			}
			var destDone = make(chan struct{})
			env := startVLESS(t, false, true, func(o *VLESSOptions) {
				need16(t, cfg.SaveTo(o.UsersFile))
				o.QuotaStore = st
				o.QuotaServer = "server"
				o.dial = func(ctx context.Context, n, address string) (net.Conn, error) {
					if strings.HasSuffix(address, ":"+portText16(alicePort)) && used.CompareAndSwap(false, true) {
						a, b := net.Pipe()
						go func() {
							defer close(destDone)
							defer b.Close()
							buf := make([]byte, 128)
							for {
								n, e := b.Read(buf)
								written.Add(int64(n))
								if e != nil {
									return
								}
							}
						}()
						if action == "short-in-flight" {
							return gateNet16{a, func(p []byte) (int, error) {
								n, e := a.Write(p[:13])
								if e != nil {
									return n, e
								}
								close(reached)
								var one [1]byte
								_, e = a.Read(one[:])
								<-unblock
								return n, e
							}}, nil
						}
						return a, nil
					}
					return (&net.Dialer{}).DialContext(ctx, n, address)
				}
				o.quotaFault = func(point, user, dir string) error {
					if user != "alice" || dir != "up" {
						return nil
					}
					if point == "reserve-reply" && action != "short-in-flight" {
						close(reached)
						<-unblock
						if action == "reserve-reply-lost" {
							return errors.New("lost reserve acknowledgement")
						}
					}
					if point == "before-settle" && action == "settlement-failed" {
						return errors.New("settlement unavailable")
					}
					if point == "settle-reply" && action == "settle-reply-lost" {
						return errors.New("lost settle acknowledgement")
					}
					return nil
				}
			})
			open := func(name, pw string, port uint16, ok bool) *testWSClient {
				cl, _ := dialWS(t, env, testWSPath, "")
				need16(t, cl.writeFrame(2, buildVLESSRequest(env.userID(name, pw), vlessCmdTCP, host, port)))
				op, b, e := cl.readFrame()
				accepted := e == nil && op == 2 && bytes.Equal(b, []byte{0, 0})
				if accepted != ok {
					cl.conn.Close()
					current, _ := config.LoadFrom(env.usersFile)
					expiry := ""
					if current != nil {
						if u := current.GetUser(name); u != nil {
							expiry = u.ExpiresAt
						}
					}
					t.Fatalf("unexpected admission: accepted=%t wanted=%t now=%s configured_expiry=%s", accepted, ok, time.Now().UTC().Format(time.RFC3339Nano), expiry)
				}
				return cl
			}
			b := open("bob", "pw-bob", bobPort, true)
			defer b.conn.Close()
			echo := func() {
				need16(t, b.writeFrame(2, []byte("probe")))
				op, v, e := b.readFrame()
				need16(t, e)
				if op != 2 || string(v) != "probe" {
					t.Fatal("Bob lost bytes or session")
				}
			}
			echo()
			a := open("alice", "pw-alice", alicePort, true)
			defer a.conn.Close()
			var idle *testWSClient
			if action == "idle-at-limit" {
				idle = open("alice", "pw-alice", bobPort, true)
				defer idle.conn.Close()
			}
			need16(t, a.writeFrame(2, bytes.Repeat([]byte("x"), 100)))
			signal16(t, reached)
			state, e := st.Read(ctx, alice)
			need16(t, e)
			opening := int64(0)
			if action == "idle-at-limit" {
				opening = (1 << 30) - 100
			}
			if state != (quota.Amount{Confirmed: opening, Pending: 100}) {
				t.Fatal("no outstanding ticket", state)
			}
			var ack chan error
			control := action
			if control == "short-in-flight" {
				control = "revoke"
			}
			if control == "password" || control == "renew" || control == "revoke" {
				cmd := exec.Command(os.Args[0], "-test.run=^TestGateConfigProcess$")
				cmd.Env = append(os.Environ(), "RV16_CONFIG="+env.usersFile, "RV16_ACTION="+control)
				need16(t, cmd.Start())
				ack = make(chan error, 1)
				go func() { ack <- cmd.Wait() }()
				until := time.Now().Add(2 * time.Second)
				for {
					l, e := config.AdmitFinite(env.usersFile, "alice")
					if e != nil {
						break
					}
					l.Close()
					if time.Now().After(until) {
						t.Fatal("fence never closed")
					}
					time.Sleep(time.Millisecond)
				}
				if action != "short-in-flight" {
					echo()
					bad := open("alice", "pw-alice", alicePort, false)
					bad.conn.Close()
					select {
					case <-ack:
						t.Fatal("acknowledged with Alice ticket held")
					default:
					}
				}
			} else if action == "shutdown" || action == "expiry" {
				if action == "shutdown" {
					need16(t, env.srv.httpSrv.Close())
				}
				a.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				_, _, e := a.readFrame()
				if e == nil {
					t.Fatal("Alice socket not cancelled")
				}
				var ne net.Error
				if errors.As(e, &ne) && ne.Timeout() {
					t.Fatal("socket only timed out")
				}
				if action == "shutdown" {
					select {
					case <-env.done:
						t.Fatal("shutdown did not join")
					default:
					}
				} else {
					echo()
				}
			}
			release()
			if ack != nil {
				select {
				case e := <-ack:
					need16(t, e)
				case <-time.After(3 * time.Second):
					t.Fatal("no acknowledgment")
				}
			}
			if action == "shutdown" {
				signal16(t, env.done)
			}
			signal16(t, destDone)
			// Destination close precedes settlement. Observe handler completion,
			// which includes both pumps and the independent settlement attempt.
			deadline := time.Now().Add(3 * time.Second)
			for {
				env.srv.sessMu.Lock()
				live := false
				for _, id := range env.srv.sessions {
					if id == env.userID("alice", "pw-alice") {
						live = true
					}
				}
				env.srv.sessMu.Unlock()
				if !live {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("session did not join settlement")
				}
				time.Sleep(time.Millisecond)
			}
			want, pend := int64(0), int64(0)
			switch action {
			case "reserve-reply-lost":
				pend = 100
			case "settlement-failed":
				pend = 100
			case "settle-reply-lost", "idle-at-limit":
				want = 100
			case "short-in-flight":
				want = 13
			}
			wantWritten := want
			if action == "settlement-failed" {
				wantWritten = 100
			}
			state, e = st.Read(ctx, alice)
			need16(t, e)
			if state != (quota.Amount{Confirmed: opening + want, Pending: pend}) || written.Load() != wantWritten {
				t.Fatalf("state=%+v destination=%d", state, written.Load())
			}
			if idle != nil {
				idle.conn.SetReadDeadline(time.Now().Add(time.Second))
				_, _, e := idle.readFrame()
				var ne net.Error
				if e == nil || (errors.As(e, &ne) && ne.Timeout()) {
					t.Fatal("empty idle session survived confirmed exhaustion")
				}
			}
			if control == "renew" {
				bad := open("alice", "pw-alice", bobPort, false)
				bad.conn.Close()
				need16(t, st.Renew(ctx, alice, "renew-A", "p2", 1<<30))
				next := open("alice", "pw-alice", bobPort, true)
				next.conn.Close()
				z, e := st.Read(ctx, quota.Key{Account: "A", Period: "p2"})
				need16(t, e)
				if z != (quota.Amount{}) {
					t.Fatal("new period charged")
				}
			}
			if control == "password" || control == "revoke" || control == "expiry" {
				if control == "expiry" {
					// Cancellation may be observed before the serialized wall-clock
					// deadline (notably after a VM clock correction). A connection
					// closed early does not prove that this account has expired.
					expires, e := time.Parse(time.RFC3339Nano, cfg.Users[0].ExpiresAt)
					need16(t, e)
					started := time.Now()
					for time.Now().Before(expires) {
						if time.Since(started) > 3*time.Second {
							t.Fatal("expiry wall clock did not advance")
						}
						time.Sleep(time.Millisecond)
					}
				}
				bad := open("alice", "pw-alice", bobPort, false)
				bad.conn.Close()
			}
			if control == "password" {
				_, e := env.srv.ReloadUsers()
				need16(t, e)
				v := open("alice", "replacement", bobPort, true)
				v.conn.Close()
			}
			if action != "shutdown" {
				echo()
			}
			t.Logf("action=%s reserved=100 Alice_destination=%d Alice_confirmed=%d Alice_pending=%d Bob_same_session=%t", action, written.Load(), state.Confirmed, state.Pending, action != "shutdown")
		})
	}
}
func portText16(p uint16) string { return strconv.Itoa(int(p)) }
