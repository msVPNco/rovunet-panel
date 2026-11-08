package coreinstall

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

// seams replaces everything that would touch systemd or Xray and records what
// the code asked for.
type seams struct {
	restarts, starts, stops, waits int
	restartErr                     []error // returned in order, then nil
	testErr                        error
}

func fakeSeams(t *testing.T) *seams {
	t.Helper()
	s := &seams{}
	oldRestart, oldStart, oldStop, oldTest, oldInstalled, oldWait := restartCore, startUnit, stopUnit, testXray, xrayInstalled, waitBetweenAttempts
	t.Cleanup(func() {
		restartCore, startUnit, stopUnit, testXray, xrayInstalled, waitBetweenAttempts = oldRestart, oldStart, oldStop, oldTest, oldInstalled, oldWait
	})
	restartCore = func(string) error {
		s.restarts++
		if len(s.restartErr) > 0 {
			err := s.restartErr[0]
			s.restartErr = s.restartErr[1:]
			return err
		}
		return nil
	}
	startUnit = func(string) error { s.starts++; return nil }
	stopUnit = func(string) error { s.stops++; return nil }
	testXray = func(string) error { return s.testErr }
	xrayInstalled = func() bool { return true }
	waitBetweenAttempts = func() { s.waits++ }
	return s
}

var (
	free    = config.UserConfig{Username: "alice", Password: "pw-a"}
	limited = config.UserConfig{Username: "carol", Password: "pw-c", QuotaGB: 5}
	hy2     = coreplan.Options{Protocol: "hysteria2", Host: "203.0.113.1", SNI: "example.org", Port: 8443, Cert: "c.pem", Key: "k.pem"}
	reality = coreplan.Options{Protocol: "reality", Host: "203.0.113.1", SNI: "www.example.com", RealityTarget: "www.example.com:443", Port: 9443}
)

func names(dir string) []string {
	var out []string
	for n := range clientNames(dir) {
		out = append(out, n)
	}
	return out
}

func TestTrafficLimitedUserIsLeftOutOfUncountedCores(t *testing.T) {
	Root = t.TempDir()
	users := []config.UserConfig{free, limited}
	newUserCore(t, "hy2", hy2, users)
	newUserCore(t, "wg", coreplan.Options{Protocol: "wireguard", Host: "203.0.113.1", Port: 51820}, users)
	newUserCore(t, "real", reality, users)

	for _, c := range []string{"hy2", "wg"} {
		if got := names(filepath.Join(Root, c)); len(got) != 1 || got[0] != "alice" {
			t.Fatalf("%s clients = %v, want only alice", c, got)
		}
	}
	if got := names(filepath.Join(Root, "real")); len(got) != 2 {
		t.Fatalf("a counted core must serve every user, got %v", got)
	}
}

func TestLimitingAUserRevokesTheirOldCredentialsAndKeepsOthers(t *testing.T) {
	Root = t.TempDir()
	s := fakeSeams(t)
	// carol first has unlimited access and keeps a Hysteria 2 profile.
	carolFree := config.UserConfig{Username: "carol", Password: "pw-c"}
	newUserCore(t, "hy2", hy2, []config.UserConfig{free, carolFree})
	old, err := os.ReadFile(filepath.Join(Root, "hy2", "clients", "carol.txt"))
	if err != nil {
		t.Fatal(err)
	}

	// She is renewed with a traffic limit and the same password.
	if err := syncCore("hy2", []config.UserConfig{free, limited}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(Root, "hy2", "clients", "carol.txt")); !os.IsNotExist(err) {
		t.Fatal("limited user still has a client file")
	}
	server, _ := os.ReadFile(filepath.Join(Root, "hy2", "server.json"))
	cred := strings.TrimPrefix(strings.SplitN(strings.TrimSpace(string(old)), "@", 2)[0], "hysteria2://")
	if cred == "" || strings.Contains(string(server), cred) {
		t.Fatalf("server.json still accepts the old credential %q", cred)
	}
	if s.restarts != 1 || s.stops != 0 {
		t.Fatalf("restarts=%d stops=%d, want the core restarted once and kept running", s.restarts, s.stops)
	}
	if got := names(filepath.Join(Root, "hy2")); len(got) != 1 || got[0] != "alice" {
		t.Fatalf("other users were disturbed: %v", got)
	}
}

func TestCoreWithNoAllowedUserIsStoppedAndStartsAgain(t *testing.T) {
	Root = t.TempDir()
	s := fakeSeams(t)
	newUserCore(t, "hy2", hy2, []config.UserConfig{free})

	if err := syncCore("hy2", []config.UserConfig{limited}); err != nil {
		t.Fatal(err)
	}
	_, st, _, _ := loadUserCore("hy2")
	if !st.Paused || s.stops != 1 || s.restarts != 0 {
		t.Fatalf("paused=%v stops=%d restarts=%d, want a stopped core", st.Paused, s.stops, s.restarts)
	}
	if st.ServerPrivate == "" || st.Seed == "" {
		t.Fatal("keys and settings must survive a pause")
	}
	if len(PendingRevocations()) != 0 {
		t.Fatal("a pause is not a pending revocation")
	}

	// The next allowed user starts it again.
	if err := syncCore("hy2", []config.UserConfig{limited, free}); err != nil {
		t.Fatal(err)
	}
	_, st, _, _ = loadUserCore("hy2")
	if st.Paused || s.starts != 1 {
		t.Fatalf("paused=%v starts=%d, want a running core", st.Paused, s.starts)
	}
}

func TestFailedRevocationIsNotReportedAsDoneAndStopsTheCore(t *testing.T) {
	Root = t.TempDir()
	s := fakeSeams(t)
	carolFree := config.UserConfig{Username: "carol", Password: "pw-c"}
	newUserCore(t, "hy2", hy2, []config.UserConfig{free, carolFree})

	s.testErr = errors.New("xray rejected the config")
	err := syncCore("hy2", []config.UserConfig{free, limited})
	if err == nil {
		t.Fatal("a revocation that could not be applied was reported as success")
	}
	if s.stops != 1 {
		t.Fatalf("stops=%d, want the core stopped so the old credential cannot be used", s.stops)
	}
	if s.waits != 0 {
		t.Fatal("a config Xray rejects cannot improve by waiting")
	}
	if p := PendingRevocations(); len(p) != 1 || !strings.HasPrefix(p[0], "hy2: ") {
		t.Fatalf("pending = %v", p)
	}
	// The file on disk is the new one: a reboot must not bring carol back.
	if _, err := os.Stat(filepath.Join(Root, "hy2", "clients", "carol.txt")); !os.IsNotExist(err) {
		t.Fatal("revoked user's client file is still there")
	}

	// Once the fault is gone the next run applies it and starts the core.
	s.testErr = nil
	if err := syncCore("hy2", []config.UserConfig{free, limited}); err != nil {
		t.Fatal(err)
	}
	if len(PendingRevocations()) != 0 || s.starts != 1 {
		t.Fatalf("pending=%v starts=%d, want it cleared and the core started", PendingRevocations(), s.starts)
	}
}

func TestRestartIsRetriedOnceThenRevocationStopsTheCore(t *testing.T) {
	Root = t.TempDir()
	s := fakeSeams(t)
	carolFree := config.UserConfig{Username: "carol", Password: "pw-c"}
	newUserCore(t, "hy2", hy2, []config.UserConfig{free, carolFree})

	// A transient failure is retried once, after a wait.
	s.restartErr = []error{errors.New("busy")}
	if err := syncCore("hy2", []config.UserConfig{free, limited}); err != nil {
		t.Fatal(err)
	}
	if s.restarts != 2 || s.waits != 1 || s.stops != 0 {
		t.Fatalf("restarts=%d waits=%d stops=%d", s.restarts, s.waits, s.stops)
	}

	// A persistent one stops the core after the second attempt.
	Root = t.TempDir()
	*s = seams{restartErr: []error{errors.New("a"), errors.New("b")}}
	newUserCore(t, "hy2", hy2, []config.UserConfig{free, carolFree})
	if err := syncCore("hy2", []config.UserConfig{free, limited}); err == nil {
		t.Fatal("persistent failure reported as success")
	}
	if s.restarts != 2 || s.stops != 1 {
		t.Fatalf("restarts=%d stops=%d, want two attempts then a stop", s.restarts, s.stops)
	}
}

func TestFailedAdditionKeepsThePreviousConfigurationRunning(t *testing.T) {
	Root = t.TempDir()
	s := fakeSeams(t)
	newUserCore(t, "hy2", hy2, []config.UserConfig{free})
	before, _ := os.ReadFile(filepath.Join(Root, "hy2", "server.json"))

	s.testErr = errors.New("xray rejected the config")
	bob := config.UserConfig{Username: "bob", Password: "pw-b"}
	if err := syncCore("hy2", []config.UserConfig{free, bob}); err == nil {
		t.Fatal("expected the failure to be reported")
	}
	after, _ := os.ReadFile(filepath.Join(Root, "hy2", "server.json"))
	if string(after) != string(before) || s.stops != 0 {
		t.Fatalf("an addition that failed must leave the running config alone (stops=%d)", s.stops)
	}
	if len(PendingRevocations()) != 0 {
		t.Fatal("no access was being removed")
	}
}
