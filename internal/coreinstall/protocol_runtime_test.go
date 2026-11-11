package coreinstall

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
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

	"github.com/msVPNco/rovunet-panel/internal/backup"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

func freeTCP(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func runtimeProcess(t *testing.T, path string, data []byte, cert string, port int) *exec.Cmd {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(XrayPath, "run", "-c", path)
	cmd.Env = append(os.Environ(), "SSL_CERT_FILE="+cert)
	var logs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("engine diagnostics: %s", logs.String())
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return cmd
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("test engine did not listen")
	return nil
}

func runtimeTLS(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	crt := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.test"}, DNSNames: []string{"example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, crt, crt, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv}), 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

// The outbound is constructed from the actual generated client URI. Traffic
// traverses two real Xray processes; the destination is an isolated loopback
// HTTP echo server. This is runtime evidence, not a phone or Internet test.
func runtimeClient(t *testing.T, link, dir, cert string) *http.Client {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(u.Port())
	auth := u.User.Username()
	obj := map[string]any{}
	q := u.Query()
	switch u.Scheme {
	case "trojan":
		obj = map[string]any{"protocol": "trojan", "settings": map[string]any{"servers": []any{map[string]any{"address": u.Hostname(), "port": port, "password": auth}}}, "streamSettings": map[string]any{"network": "raw", "security": "tls", "tlsSettings": map[string]any{"serverName": q.Get("sni"), "alpn": []string{"http/1.1"}, "certificates": []any{map[string]any{"certificateFile": cert, "usage": "verify"}}}}}
	case "vless":
		obj = map[string]any{"protocol": "vless", "settings": map[string]any{"vnext": []any{map[string]any{"address": u.Hostname(), "port": port, "users": []any{map[string]any{"id": auth, "encryption": q.Get("encryption")}}}}}, "streamSettings": map[string]any{"network": "raw", "security": "none"}}
	case "ss":
		decoded, err := base64.RawURLEncoding.DecodeString(auth)
		if err != nil {
			t.Fatal(err)
		}
		method, password, ok := strings.Cut(string(decoded), ":")
		if !ok {
			t.Fatal("invalid SS2022 URI")
		}
		obj = map[string]any{"protocol": "shadowsocks", "settings": map[string]any{"servers": []any{map[string]any{"address": u.Hostname(), "port": port, "method": method, "password": password}}}}
	default:
		t.Fatal("unexpected URI scheme")
	}
	socks := freeTCP(t)
	doc := map[string]any{"log": map[string]any{"loglevel": "debug"}, "inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": socks, "protocol": "socks", "settings": map[string]any{"auth": "noauth"}}}, "outbounds": []any{obj}}
	b, _ := json.Marshal(doc)
	runtimeProcess(t, filepath.Join(dir, fmtPort(socks)+".json"), b, cert, socks)
	proxy, _ := url.Parse("socks5://127.0.0.1:" + fmtPort(socks))
	return &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}}
}

func fmtPort(p int) string { return strconv.Itoa(p) }

func TestManagedProtocolsRuntime(t *testing.T) {
	engine := os.Getenv("XRAY")
	if engine == "" {
		t.Skip("set XRAY to the checksum-verified pinned engine")
	}
	oldRoot, oldXray, oldUsage, oldEpoch, oldGenerate := Root, XrayPath, usage.Dir, runEpoch, generateUserBundle
	t.Cleanup(func() {
		Root, XrayPath, usage.Dir, runEpoch, generateUserBundle = oldRoot, oldXray, oldUsage, oldEpoch, oldGenerate
	})
	XrayPath = engine
	runEpoch = func(string) (string, error) { return "runtime-test", nil }
	testRoot := t.TempDir()
	usage.Dir = filepath.Join(testRoot, "initial-usage")
	payload := bytes.Repeat([]byte("rovunet-runtime-payload-"), 16384)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hold" {
			conn, buf, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: test-echo\r\n\r\n")
			_ = buf.Flush()
			go func() { defer conn.Close(); _, _ = io.Copy(conn, conn) }()
			return
		}
		body, _ := io.ReadAll(r.Body)
		_, _ = w.Write(body)
	}))
	defer destination.Close()
	generateUserBundle = func(o coreplan.Options, st *coreplan.State, clients []coreplan.Client) (coreplan.Bundle, error) {
		b, err := coreplan.GenerateFor(o, st, clients)
		if err != nil {
			return b, err
		}
		var root map[string]any
		_ = json.Unmarshal(b.Server, &root)
		// Test-only egress exception: the pinned engine otherwise blocks
		// loopback destinations. Credentials, listener, API and accounting
		// remain exactly those generated by production code.
		root["outbounds"].([]any)[0].(map[string]any)["settings"] = map[string]any{"finalRules": []any{map[string]any{"action": "allow", "ip": []string{"127.0.0.1/32"}}}}
		root["log"] = map[string]any{"loglevel": "debug"}
		b.Server, err = json.Marshal(root)
		return b, err
	}
	for _, protocol := range []string{"ss2022", "trojan", "vlessenc"} {
		t.Run(protocol, func(t *testing.T) {
			epoch := "first-" + protocol
			runEpoch = func(string) (string, error) { return epoch, nil }
			Root = t.TempDir()
			previousTotals := usage.Totals()
			cert, key := runtimeTLS(t, t.TempDir())
			o := coreplan.Options{Protocol: protocol, Host: "127.0.0.1", Port: freeTCP(t), SNI: "example.test", Cert: cert, Key: key}
			alice := config.UserConfig{Username: "alice", Password: "fixture-a"}
			bob := config.UserConfig{Username: "bob", Password: "fixture-b"}
			newUserCore(t, protocol, o, []config.UserConfig{alice, bob})
			dir := filepath.Join(Root, protocol)
			server, _ := os.ReadFile(filepath.Join(dir, "server.json"))
			serverProcess := runtimeProcess(t, filepath.Join(dir, "server.json"), server, cert, o.Port)
			clients := map[string]*http.Client{}
			for _, u := range []config.UserConfig{alice, bob} {
				profiles, err := Profiles(u)
				if err != nil || len(profiles) != 1 {
					t.Fatalf("provisioned profiles=%d err=%v", len(profiles), err)
				}
				clients[u.Username] = runtimeClient(t, profiles[0].Link, t.TempDir(), cert)
			}
			request := func(client *http.Client) error {
				response, err := client.Post(destination.URL, "application/octet-stream", bytes.NewReader(payload))
				if err != nil {
					return err
				}
				defer response.Body.Close()
				body, err := io.ReadAll(response.Body)
				if err != nil {
					return err
				}
				if !bytes.Equal(body, payload) {
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			for name, client := range clients {
				if err := request(client); err != nil {
					t.Fatalf("%s data round trip: %v", name, err)
				}
			}
			if err := collectCore(protocol); err != nil {
				t.Fatal(err)
			}
			before := usage.Totals()
			for _, name := range []string{"alice", "bob"} {
				if before[name]-previousTotals[name] < int64(2*len(payload)) {
					t.Fatalf("%s per-user upload/download not counted: %d", name, before[name])
				}
			}
			if err := collectCore(protocol); err != nil {
				t.Fatal(err)
			}
			if got := usage.Totals(); got["alice"] != before["alice"] || got["bob"] != before["bob"] {
				t.Fatal("repeated API read double counted")
			}
			holdClient := &http.Client{Transport: clients["alice"].Transport}
			held, err := holdClient.Get(destination.URL + "/hold")
			if err != nil {
				t.Fatal(err)
			}
			stream, ok := held.Body.(io.ReadWriteCloser)
			if !ok {
				t.Fatal("upgrade stream unavailable")
			}
			defer stream.Close()
			probe := func() bool {
				result := make(chan bool, 1)
				go func() {
					if _, err := stream.Write([]byte("ping")); err != nil {
						result <- false
						return
					}
					var b [4]byte
					_, err := io.ReadFull(stream, b[:])
					result <- err == nil && string(b[:]) == "ping"
				}()
				select {
				case ok := <-result:
					return ok
				case <-time.After(2 * time.Second):
					_ = stream.Close()
					return false
				}
			}
			if !probe() {
				t.Fatal("active stream did not pass data before revocation")
			}
			alice.ExpiresAt = "2000-01-01T00:00:00Z"
			if err := syncCore(protocol, []config.UserConfig{alice, bob}); err != nil {
				t.Fatal(err)
			}
			if err := request(clients["alice"]); err == nil {
				t.Fatal("expired account opened a new session")
			}
			if err := request(clients["bob"]); err != nil {
				t.Fatalf("healthy account interrupted: %v", err)
			}
			if probe() {
				t.Log("LIMITATION OBSERVED: HandlerService removal leaves this established stream alive")
			}
			alice.ExpiresAt = ""
			if err := syncCore(protocol, []config.UserConfig{alice, bob}); err != nil {
				t.Fatal(err)
			}
			if err := request(clients["alice"]); err != nil {
				t.Fatal("renewed credential did not work")
			}
			alice.QuotaGB = 1
			if err := syncCore(protocol, []config.UserConfig{alice, bob}); err != nil {
				t.Fatal(err)
			}
			if err := request(clients["alice"]); err == nil {
				t.Fatal("traffic-limited account opened a session on an ineligible protocol")
			}
			if profiles, err := Profiles(alice); err != nil || len(profiles) != 0 {
				t.Fatalf("traffic-limited account exported %d ineligible profiles: %v", len(profiles), err)
			}
			if err := syncCore(protocol, []config.UserConfig{bob}); err != nil {
				t.Fatal(err)
			}
			if err := request(clients["alice"]); err == nil {
				t.Fatal("deleted account opened a new session")
			}
			bob.Password = "new-fixture-password"
			if err := syncCore(protocol, []config.UserConfig{bob}); err != nil {
				t.Fatal(err)
			}
			if err := request(clients["bob"]); err == nil {
				t.Fatal("old password still opened a new session")
			}
			updated, err := Profiles(bob)
			if err != nil || len(updated) != 1 {
				t.Fatal("updated private profile unavailable")
			}
			if err := request(runtimeClient(t, updated[0].Link, t.TempDir(), cert)); err != nil {
				t.Fatalf("rotated password: %v", err)
			}
			actualAPI := runUserAPI
			lostReply := true
			runUserAPI = func(endpoint, action string, args ...string) ([]byte, error) {
				out, err := actualAPI(endpoint, action, args...)
				if action == "adu" && err == nil && lostReply {
					lostReply = false
					return nil, errors.New("simulated lost API reply")
				}
				return out, err
			}
			defer func() { runUserAPI = actualAPI }()
			carol := config.UserConfig{Username: "carol", Password: "fixture-c", ExpiresAt: "2099-01-01T00:00:00Z"}
			if err := syncCore(protocol, []config.UserConfig{bob, carol}); err == nil {
				t.Fatal("lost API reply was reported as a confirmed addition")
			}
			if len(PendingRevocations()) == 0 {
				t.Fatal("unconfirmed addition was not recorded for retry")
			}
			if err := syncCore(protocol, []config.UserConfig{bob, carol}); err != nil {
				t.Fatalf("retry after a partial API operation: %v", err)
			}
			if len(PendingRevocations()) != 0 {
				t.Fatal("successful retry left a pending update")
			}
			profiles, err := Profiles(carol)
			if err != nil || len(profiles) != 1 {
				t.Fatal("retried timed account has no applied profile")
			}
			if err := request(runtimeClient(t, profiles[0].Link, t.TempDir(), cert)); err != nil {
				t.Fatal("retried timed account did not pass data")
			}
			if err := collectCore(protocol); err != nil {
				t.Fatal(err)
			}
			savedTotals := usage.Totals()
			history, err := usage.History()
			if err != nil {
				t.Fatal(err)
			}
			bundle := &backup.Bundle{}
			if err := bundle.AddCores(Root); err != nil {
				t.Fatal(err)
			}
			if err := serverProcess.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			_ = serverProcess.Wait()
			Root = t.TempDir()
			if _, err := bundle.RestoreCores(Root); err != nil {
				t.Fatal(err)
			}
			usage.Dir = filepath.Join(testRoot, "restored-usage-"+protocol)
			if err := usage.RestoreHistory(history); err != nil {
				t.Fatal(err)
			}
			if err := usage.Restore(savedTotals); err != nil {
				t.Fatal(err)
			}
			if got := usage.Totals(); got["alice"] != savedTotals["alice"] || got["bob"] != savedTotals["bob"] {
				t.Fatal("backup restore lost or double-counted persisted usage")
			}
			if err := Prepare(protocol, []config.UserConfig{bob, carol}); err != nil {
				t.Fatal(err)
			}
			dir = filepath.Join(Root, protocol)
			server, err = os.ReadFile(filepath.Join(dir, "server.json"))
			if err != nil {
				t.Fatal(err)
			}
			runtimeProcess(t, filepath.Join(dir, "server.json"), server, cert, o.Port)
			if err := saveApplied(dir); err != nil {
				t.Fatal(err)
			}
			if err := request(runtimeClient(t, profiles[0].Link, t.TempDir(), cert)); err != nil {
				t.Fatal("restored timed account did not pass data")
			}
			if err := request(clients["alice"]); err == nil {
				t.Fatal("removed credential returned after restore/restart")
			}
			epoch = "restarted-" + protocol
			if err := collectCore(protocol); err == nil {
				t.Fatal("a process epoch change concealed an unsampled interval")
			}
			if got := usage.Totals()["bob"]; got < savedTotals["bob"] {
				t.Fatal("restart erased prior usage")
			}
			t.Log("private core/TLS backup restore and process restart passed data; totals survived and the epoch gap remained explicit")
			t.Log("two distinct accounts, bidirectional data, per-user counters, idempotent sampling and live removal passed")
		})
	}
	if got := usage.Totals()["alice"]; got < int64(6*len(payload)) {
		t.Fatalf("shared account usage across three engines is too low: %d", got)
	}
	t.Log("one shared account total includes all three protocol sources without interface/NAT counters")
}
