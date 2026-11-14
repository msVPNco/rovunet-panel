package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/certs"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Opt-in only: the runner supplies an isolated network namespace with an echo
// address on loopback. No production routing or target exception is changed.
func TestPinnedXrayFiniteBridges(t *testing.T) {
	engine := os.Getenv("ROVUNET_TEST_XRAY")
	if engine == "" {
		t.Skip("isolated pinned-runtime acceptance")
	}
	for _, protocol := range []string{"reality", "xhttp", "tlsvision", "xhttp-reality"} {
		t.Run(protocol, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			db := filepath.Join(dir, "db")
			store, e := quota.Create(ctx, db, "server", "store")
			need16(t, e)
			defer store.Close()
			key := quota.Key{Account: "A", Period: "p1"}
			need16(t, store.NewPeriod(ctx, key, 1<<30))
			need16(t, store.ImportOpening(ctx, quota.Checkpoint{Server: "server", Store: "store", Account: "A", Period: "p1", ID: "opening", OpeningOrigin: "fixture", Proof: "synthetic", Opening: (1 << 30) - 100}))
			need16(t, store.FinishBootstrap(db))
			cfg := &config.Config{ServerID: "server", Users: []config.UserConfig{{Username: "alice", Password: "pw-alice", AccountID: "A", PeriodID: "p1", QuotaGB: 1, SQLiteMeter: &config.SQLiteBinding{Directory: db, Store: "store"}}, {Username: "bob", Password: "pw-bob", AccountID: "B", PeriodID: "p1"}}}
			env := startVLESS(t, false, true, func(o *VLESSOptions) {
				need16(t, cfg.SaveTo(o.UsersFile))
				o.QuotaStore = store
				o.QuotaServer = "server"
			})
			_, p, _ := net.SplitHostPort(env.addr)
			port, _ := strconv.Atoi(p)
			cert, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
			need16(t, certs.GenerateSelfSigned(cert, keyFile, "test.example.org"))
			target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("fixture")) }))
			target.EnableHTTP2 = true
			target.StartTLS()
			defer target.Close()
			state, e := coreplan.NewState()
			need16(t, e)
			state.APISocket = filepath.Join(dir, "xray-api.sock")
			o := coreplan.Options{Protocol: protocol, Host: "test.example.org", SNI: "test.example.org", Port: freePort25(t), Cert: cert, Key: keyFile, RealityTarget: target.Listener.Addr().String()}
			// Generate before and after attachment: public links must be identical.
			users := []coreplan.Client{{Name: "alice", Secret: "pw-alice"}, {Name: "bob", Secret: "pw-bob"}}
			before, e := coreplan.GenerateFor(o, &state, users)
			need16(t, e)
			id := env.userID("alice", "pw-alice")
			o.QuotaBridge = &coreplan.NativeQuotaBridge{Port: port, Path: testWSPath}
			o.QuotaClients = map[string]string{"alice": fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])}
			bundle, e := coreplan.GenerateFor(o, &state, users)
			need16(t, e)
			for i := range before.Clients {
				if before.Clients[i] != bundle.Clients[i] {
					t.Fatal("public profile changed on bridge attachment")
				}
			}
			startXray25(t, engine, dir, "server", bundle.Server, o.Port)
			echo, e := net.Listen("tcp", "8.8.8.8:0")
			need16(t, e)
			defer echo.Close()
			go func() {
				for {
					c, e := echo.Accept()
					if e != nil {
						return
					}
					go func() { defer c.Close(); io.Copy(c, c) }()
				}
			}()
			dest := echo.Addr().(*net.TCPAddr).Port
			ports := []int{freePort25(t), freePort25(t)}
			for i, link := range bundle.Clients {
				u, e := url.Parse(link)
				need16(t, e)
				q := u.Query()
				user := map[string]any{"id": u.User.Username(), "encryption": "none"}
				if q.Get("flow") != "" {
					user["flow"] = q.Get("flow")
				}
				stream := map[string]any{"network": "raw", "security": q.Get("security")}
				if q.Get("security") == "reality" {
					stream["realitySettings"] = map[string]any{"serverName": o.SNI, "fingerprint": "chrome", "password": q.Get("pbk"), "shortId": q.Get("sid")}
				} else {
					stream["tlsSettings"] = map[string]any{"serverName": o.SNI, "disableSystemRoot": true, "certificates": []any{map[string]any{"certificateFile": cert, "usage": "verify"}}}
				}
				if protocol == "xhttp" || protocol == "xhttp-reality" {
					stream["network"] = "xhttp"
					stream["xhttpSettings"] = map[string]any{"path": q.Get("path"), "mode": "auto"}
				}
				doc := map[string]any{"log": map[string]any{"loglevel": "warning"}, "inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": ports[i], "protocol": "socks", "settings": map[string]any{"auth": "noauth"}}}, "outbounds": []any{map[string]any{"protocol": "vless", "settings": map[string]any{"vnext": []any{map[string]any{"address": "127.0.0.1", "port": o.Port, "users": []any{user}}}}, "streamSettings": stream}}}
				b, _ := json.Marshal(doc)
				startXray25(t, engine, dir, fmt.Sprintf("client%d", i), b, ports[i])
			}
			bob := socksEcho25(t, ports[1], dest)
			defer bob.Close()
			alice := socksEcho25(t, ports[0], dest)
			defer alice.Close()
			need16(t, alice.SetDeadline(time.Now().Add(5*time.Second)))
			_, e = alice.Write(make([]byte, 50))
			need16(t, e)
			b := make([]byte, 50)
			_, e = io.ReadFull(alice, b)
			need16(t, e)
			started := time.Now()
			need16(t, alice.SetReadDeadline(time.Now().Add(2*time.Second)))
			_, e = alice.Read(make([]byte, 1))
			if e == nil {
				t.Fatal("open finite session not closed")
			}
			if n, ok := e.(net.Error); ok && n.Timeout() {
				t.Fatal("cutoff timed out")
			}
			cutoff := time.Since(started)
			amount, e := store.Read(ctx, key)
			need16(t, e)
			if amount.Confirmed != 1<<30 || amount.Pending != 0 {
				t.Fatal("budget mismatch", amount)
			}
			removed, apiErr := exec.Command(engine, "api", "rmu", "-s", "unix://"+state.APISocket, "-t", "5", "-tag=proxy", "alice").CombinedOutput()
			if apiErr != nil || !strings.Contains(string(removed), "Removed 1 user(s) in total.") {
				t.Fatalf("actual per-user removal failed: %v %s", apiErr, removed)
			}
			need16(t, bob.SetDeadline(time.Now().Add(3*time.Second)))
			_, e = bob.Write([]byte("unaffected"))
			need16(t, e)
			b = make([]byte, 10)
			_, e = io.ReadFull(bob, b)
			need16(t, e)
			if string(b) != "unaffected" {
				t.Fatal("second account interrupted")
			}
			t.Logf("%s: preserved links; 50 upload + 50 download; overshoot=0; closed after read in %s; other user's held stream alive", protocol, cutoff)
		})
	}
}
func freePort25(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	need16(t, e)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
func startXray25(t *testing.T, engine, dir, name string, b []byte, port int) {
	t.Helper()
	path := filepath.Join(dir, name+".json")
	need16(t, os.WriteFile(path, b, 0600))
	f, e := os.Create(filepath.Join(dir, name+".log"))
	need16(t, e)
	c := exec.Command(engine, "run", "-c", path)
	c.Stdout = f
	c.Stderr = f
	need16(t, c.Start())
	t.Cleanup(func() { c.Process.Kill(); c.Wait(); f.Close() })
	for i := 0; i < 100; i++ {
		s, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if e == nil {
			s.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("pinned Xray listener unavailable")
}
func socksEcho25(t *testing.T, port, dest int) net.Conn {
	t.Helper()
	s, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	need16(t, e)
	need16(t, s.SetDeadline(time.Now().Add(5*time.Second)))
	_, e = s.Write([]byte{5, 1, 0})
	need16(t, e)
	b := make([]byte, 2)
	_, e = io.ReadFull(s, b)
	need16(t, e)
	if b[1] != 0 {
		t.Fatal("SOCKS auth rejected")
	}
	_, e = s.Write([]byte{5, 1, 0, 1, 8, 8, 8, 8, byte(dest >> 8), byte(dest)})
	need16(t, e)
	b = make([]byte, 10)
	_, e = io.ReadFull(s, b)
	need16(t, e)
	if b[1] != 0 || b[3] != 1 {
		t.Fatal("SOCKS connect rejected")
	}
	return s
}
