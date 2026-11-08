package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCountersPersistAndSum(t *testing.T) {
	Dir = t.TempDir()
	a := NewCounter("vless")
	a.Add("alice", 100)
	a.Add("alice", 50)
	a.Add("bob", 7)
	a.Add("", 999)    // no user: not counted
	a.Add("carol", 0) // nothing moved
	if err := a.Flush(); err != nil {
		t.Fatal(err)
	}
	b := NewCounter("socks5")
	b.Add("alice", 25)
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}

	got := Totals()
	if got["alice"] != 175 || got["bob"] != 7 || len(got) != 2 {
		t.Fatalf("totals %v", got)
	}

	// A restarted server continues from its saved totals.
	again := NewCounter("vless")
	again.Add("alice", 5)
	if err := again.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := Totals()["alice"]; got != 180 {
		t.Fatalf("after restart alice=%d, want 180", got)
	}
}

func TestRestoreAddsOnlyWhatIsMissing(t *testing.T) {
	Dir = t.TempDir()
	c := NewCounter("vless")
	c.Add("alice", 100)
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}
	saved := map[string]int64{"alice": 300, "bob": 50}
	for i := 0; i < 2; i++ { // twice: must not double
		if err := Restore(saved); err != nil {
			t.Fatal(err)
		}
		got := Totals()
		if got["alice"] != 300 || got["bob"] != 50 {
			t.Fatalf("round %d: totals %v", i, got)
		}
	}
	// A server that already counted more keeps its own number.
	c.Add("alice", 500)
	_ = c.Flush()
	if err := Restore(saved); err != nil {
		t.Fatal(err)
	}
	if got := Totals()["alice"]; got != 600 {
		t.Fatalf("alice=%d, want 600", got)
	}
}

func TestTotalsCheckedReportsBrokenFiles(t *testing.T) {
	Dir = t.TempDir()
	defer func() { Dir = "/var/lib/rovunet/usage" }()
	if _, err := TotalsChecked(); err != nil {
		t.Fatalf("an empty directory is not an error: %v", err)
	}
	_ = os.WriteFile(filepath.Join(Dir, "vless.json"), []byte(`{"a":5}`), 0644)
	_ = os.WriteFile(filepath.Join(Dir, "xray.json"), []byte(`{"a":`), 0644)
	total, err := TotalsChecked()
	if err == nil || total["a"] != 5 {
		t.Fatalf("a broken file must be reported, the rest summed: %v %v", total, err)
	}
	if Totals()["a"] != 5 {
		t.Fatal("Totals keeps summing what it can read")
	}
}

func TestFailedReadKeepsLastCompleteSample(t *testing.T) {
	Dir = t.TempDir()
	defer func() { Dir = "/var/lib/rovunet/usage" }()
	if _, ok := LastCollected(); ok {
		t.Fatal("nothing recorded yet")
	}
	t1 := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(5 * time.Minute)
	if err := MarkCollected(t1, true); err != nil {
		t.Fatal(err)
	}
	if err := MarkCollected(t2, false); err != nil {
		t.Fatal(err)
	}
	c, ok := LastCollected()
	if !ok || !c.At.Equal(t1) || !c.Tried.Equal(t2) || c.Complete {
		t.Fatalf("got %+v", c)
	}
	if err := MarkCollected(t2.Add(5*time.Minute), true); err != nil {
		t.Fatal(err)
	}
	if c, _ := LastCollected(); !c.At.Equal(t2.Add(5*time.Minute)) || !c.Complete {
		t.Fatalf("a complete read moves the sample time: %+v", c)
	}
	if _, err := os.Stat(filepath.Join(Dir, "collected.state")); err != nil {
		t.Fatal(err)
	}
	if _, err := TotalsChecked(); err != nil {
		t.Fatalf("the state file is not a totals file: %v", err)
	}
}
