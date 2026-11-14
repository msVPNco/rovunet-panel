package quotaledger

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSharedProcesses(t *testing.T) {
	if dir := os.Getenv("QUOTA_CHILD"); dir != "" {
		store := Store{dir}
		for i := 0; i < 20; i++ {
			id := fmt.Sprintf("%s-%d", os.Getenv("QUOTA_WORKER"), i)
			err := store.Reserve("alice", "p1", id, 100)
			if errors.Is(err, ErrBudget) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Settle("alice", "p1", id, 75); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	dir := privateDir(t)
	store := Store{dir}
	if err := store.Open("alice", "p1", 5000, 1234); err != nil {
		t.Fatal(err)
	}
	if err := store.Open("bob", "p1", 9000, 234); err != nil {
		t.Fatal(err)
	}
	var children []*exec.Cmd
	for i := 0; i < 4; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSharedProcesses$")
		cmd.Env = append(os.Environ(), "QUOTA_CHILD="+dir, fmt.Sprintf("QUOTA_WORKER=%d", i))
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, cmd)
	}
	for _, cmd := range children {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	state, err := store.Read("alice", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Charged > 5000 || state.Charged < 4900 {
		t.Fatalf("bad bound %d", state.Charged)
	}
	total := int64(1234)
	for _, ticket := range state.Tickets {
		if ticket.Actual == nil {
			t.Fatal("unsettled transfer")
		}
		total += *ticket.Actual
	}
	if total != state.Charged {
		t.Fatal("settlement mismatch")
	}
	bob, _ := store.Read("bob", "p1")
	if bob.Charged != 234 {
		t.Fatal("other account changed")
	}
	t.Logf("four processes, charged=%d, settled transfers=%d, bob=%d", state.Charged, len(state.Tickets), bob.Charged)
}

func TestCrashAndReplay(t *testing.T) {
	if dir := os.Getenv("QUOTA_CRASH"); dir != "" {
		store := Store{dir}
		if err := store.Reserve("alice", "p1", "uncertain", 321); err != nil {
			t.Fatal(err)
		}
		os.Exit(23)
	}
	dir := privateDir(t)
	store := Store{dir}
	if err := store.Open("alice", "p1", 1000, 123); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashAndReplay$")
	cmd.Env = append(os.Environ(), "QUOTA_CRASH="+dir)
	if err := cmd.Run(); err == nil {
		t.Fatal("expected abrupt child exit")
	}
	reopened := Store{dir}
	state, err := reopened.Read("alice", "p1")
	if err != nil || state.Charged != 444 {
		t.Fatalf("lost reservation %v %+v", err, state)
	}
	if err := reopened.Reserve("alice", "p1", "uncertain", 321); !errors.Is(err, ErrReplay) {
		t.Fatal("replay reauthorized transfer")
	}
	if err := reopened.Settle("alice", "p1", "uncertain", 300); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Settle("alice", "p1", "uncertain", 300); err != nil {
		t.Fatal(err)
	}
	state, _ = reopened.Read("alice", "p1")
	if state.Charged != 423 {
		t.Fatal("settlement counted twice")
	}
}

func TestRenewRestoreExternalAndStaleWriter(t *testing.T) {
	store := Store{privateDir(t)}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(store.Open("alice", "p1", 1000, 100))
	must(store.Observe("alice", "p1", "xray-epoch1", 200))
	must(store.Observe("alice", "p1", "xray-epoch1", 200))
	stale := store
	must(store.Observe("alice", "p1", "xray-epoch1", 250))
	must(store.Reserve("alice", "p1", "native-pending", 50))
	must(store.RaiseFloor("alice", "p1", 200))
	state, _ := store.Read("alice", "p1")
	if state.Charged != 400 {
		t.Fatal("restore lowered usage or external replay doubled")
	}
	if err := store.Observe("alice", "p1", "xray-epoch1", 1); err == nil {
		t.Fatal("source reset invented zero")
	}
	must(store.Renew("alice", "p1", "p2", 1000))
	if err := stale.Reserve("alice", "p1", "stale", 10); !errors.Is(err, ErrPeriod) {
		t.Fatal("old writer revived period")
	}
	must(stale.Settle("alice", "p1", "native-pending", 25))
	old, _ := store.Read("alice", "p1")
	current, _ := store.Read("alice", "p2")
	if !old.Closed || old.Charged != 375 || current.Charged != 0 {
		t.Fatal("old settlement crossed period")
	}
	if err := store.Open("alice", "p1", 1000, 0); !errors.Is(err, ErrPeriod) {
		t.Fatal("old initialization revived account")
	}
}

func TestCorruptionAndSymlink(t *testing.T) {
	dir := privateDir(t)
	store := Store{dir}
	p := filepath.Join(dir, "ledger.json")
	if err := os.WriteFile(p, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Open("a", "p", 100, 0); err == nil {
		t.Fatal("corruption became empty ledger")
	}
	if string(mustRead(t, p)) != "damaged" {
		t.Fatal("corrupt input overwritten")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	os.WriteFile(target, []byte("private"), 0600)
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	if err := store.Open("a", "p", 100, 0); err == nil {
		t.Fatal("followed ledger symlink")
	}
}
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func privateDir(t *testing.T) string {
	t.Helper()
	p := t.TempDir()
	if err := os.Chmod(p, 0700); err != nil {
		t.Fatal(err)
	}
	return p
}
