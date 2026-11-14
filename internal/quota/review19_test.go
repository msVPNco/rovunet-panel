package quota

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPairAtomicOwnership(t *testing.T) {
	ctx, s, _, k := newTestStore(t)
	must(t, s.ObservePair(ctx, k, "one", "xray", "epoch", 100, 200, 120, 230, 1000))
	must(t, s.ObservePair(ctx, k, "one", "xray", "epoch", 100, 200, 120, 230, 1000))
	a, e := s.Read(ctx, k)
	must(t, e)
	if a.Confirmed != 50 {
		t.Fatal(a)
	}
	if s.ObservePair(ctx, k, "bad", "xray", "epoch", 100, 200, 140, 220, 1001) == nil {
		t.Fatal("accepted down regression")
	}
	b, e := s.Read(ctx, k)
	must(t, e)
	if a != b {
		t.Fatal("half sample committed", b)
	}
	if s.ReserveNative(ctx, k, "native", "up", 1) == nil {
		t.Fatal("duplicate authority")
	}
	_, s2, _, k2 := newTestStore(t)
	must(t, s2.ReserveNative(ctx, k2, "T", "up", 1))
	if s2.ObservePair(ctx, k2, "X", "xray", "e", 0, 0, 1, 2, 1000) == nil {
		t.Fatal("native also sampled")
	}
}
func TestExplicitBootstrapResume(t *testing.T) {
	ctx := context.Background()
	d := filepath.Join(t.TempDir(), "db")
	s, e := Create(ctx, d, "S", "D")
	must(t, e)
	k := Key{"A", "P"}
	must(t, s.NewPeriod(ctx, k, 1000))
	must(t, s.Close())
	if _, e = Open(ctx, d, "S", "D"); e == nil {
		t.Fatal("opened unfinished")
	}
	r, e := ResumeBootstrap(ctx, d, "S", "D")
	must(t, e)
	must(t, r.NewPeriod(ctx, k, 1000))
	must(t, r.ImportOpening(ctx, Checkpoint{"S", "D", "A", "P", "I", "O", "synthetic disjoint", 10}))
	must(t, r.FinishBootstrap(d))
	r.Close()
	r, e = Open(ctx, d, "S", "D")
	must(t, e)
	defer r.Close()
	a, e := r.Read(ctx, k)
	must(t, e)
	if a.Confirmed != 10 {
		t.Fatal(a)
	}
	if _, e = ResumeBootstrap(ctx, filepath.Join(t.TempDir(), "missing"), "S", "D"); e == nil {
		t.Fatal("invented DB")
	}
	bad := filepath.Join(t.TempDir(), "writable")
	must(t, os.Mkdir(bad, 0777))
	must(t, os.Chmod(bad, 0777))
	if _, e = Create(ctx, filepath.Join(bad, "db"), "S", "D"); e == nil {
		t.Fatal("unsafe ancestor accepted")
	}
}

func TestFreshPeriodExclusiveBudget(t *testing.T) {
	ctx, s, _, k := newTestStore(t)
	must(t, s.ReserveNative(ctx, k, "ticket", "up", 100))
	if e := s.CarryPeriod(ctx, k, "transfer", "p2"); e == nil {
		t.Fatal("transferred uncertain pending")
	}
	must(t, s.Settle(ctx, k, "ticket", 40))
	must(t, s.CarryPeriod(ctx, k, "transfer", "p2"))
	must(t, s.CarryPeriod(ctx, k, "transfer", "p2"))
	if e := s.ReserveNative(ctx, k, "old", "up", 1); e == nil {
		t.Fatal("old still admitted")
	}
	next := Key{k.Account, "p2"}
	a, e := s.Read(ctx, next)
	must(t, e)
	if a != (Amount{Confirmed: 40}) {
		t.Fatal(a)
	}
	must(t, s.ReserveNative(ctx, next, "new", "up", 960))
	must(t, s.Settle(ctx, next, "new", 960))
	if e = s.ReserveNative(ctx, next, "extra", "down", 1); e == nil {
		t.Fatal("double budget")
	}
	old, e := s.Read(ctx, k)
	must(t, e)
	if old.Confirmed != 40 {
		t.Fatal("history rewritten")
	}
}
