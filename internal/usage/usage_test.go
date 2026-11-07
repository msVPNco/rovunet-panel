package usage

import (
	"testing"
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
