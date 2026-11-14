package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
"os/exec"

	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func must(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func newTestStore(t *testing.T) (context.Context, *Store, string, Key) {
	t.Helper()
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "quota")
	s, e := Create(ctx, dir, "test-server", "test-store")
	must(t, e)
	t.Cleanup(func() { s.Close() })
	k := Key{"alice", "p1"}
	must(t, s.NewPeriod(ctx, k, 1000))
	must(t, s.FinishBootstrap(dir))
	return ctx, s, dir, k
}
func cp(k Key) Checkpoint {
	return Checkpoint{"test-server", "test-store", k.Account, k.Period, "opening-1", "prior-cutover", "verified synthetic disjoint prefix", 200}
}
func TestPositiveOrders(t *testing.T) {
	for _, n := range []int64{0, 40, 100} {
		for _, restoreFirst := range []bool{true, false} {
			t.Run(fmt.Sprintf("%d-%t", n, restoreFirst), func(t *testing.T) {
				ctx, s, dir, k := newTestStore(t)
				must(t, s.Reserve(ctx, k, "T", "up", 100))
				if restoreFirst {
					must(t, s.ImportOpening(ctx, cp(k)))
				}
				must(t, s.Settle(ctx, k, "T", n))
				if !restoreFirst {
					must(t, s.ImportOpening(ctx, cp(k)))
				}
				must(t, s.Settle(ctx, k, "T", n))
				must(t, s.ImportOpening(ctx, cp(k)))
				reopen, e := Open(ctx, dir, "test-server", "test-store")
				must(t, e)
				defer reopen.Close()
				for _, actual := range []*int64{nil, &n} {
					snap := TicketSnapshot{"test-server", "test-store", "alice", "p1", fmt.Sprintf("snap-%t", actual == nil), []TicketProof{{"T", "up", 100, actual}}}
					must(t, reopen.MergeTickets(ctx, snap))
					must(t, reopen.MergeTickets(ctx, snap))
				}
				a, e := reopen.Read(ctx, k)
				must(t, e)
				if a != (Amount{200 + n, 0}) {
					t.Fatal(a)
				}
				if !errors.Is(reopen.Reserve(ctx, k, "T", "up", 100), ErrReplay) {
					t.Fatal("duplicate authorized")
				}
				if !errors.Is(reopen.Reserve(ctx, k, "T", "down", 100), ErrConflict) {
					t.Fatal("changed receipt accepted")
				}
				must(t, reopen.Renew(ctx, k, "r", "p2", 500))
				if err := s.Reserve(ctx, k, "late", "up", 1); err == nil {
					t.Fatal("old writer admitted")
				}
				must(t, s.Settle(ctx, k, "T", n))
				b, e := s.Read(ctx, Key{"alice", "p2"})
				must(t, e)
				if b != (Amount{}) {
					t.Fatal(b)
				}
			})
		}
	}
}
func TestBackupSourcesAndFailures(t *testing.T) {
	ctx, s, dir, k := newTestStore(t)
	must(t, s.Observe(ctx, k, "sample", "core", "epoch1", "up", 100, 150, 1000, false))
	must(t, s.Observe(ctx, k, "sample", "core", "epoch1", "up", 100, 150, 1000, false))
	if s.Observe(ctx, k, "older", "core", "epoch1", "up", 100, 140, 1001, false) == nil {
		t.Fatal("decrease")
	}
	must(t, s.Reserve(ctx, k, "T", "down", 200))
	dst := filepath.Join(t.TempDir(), "backup")
	must(t, s.Backup(ctx, dst))
	restored, e := Open(ctx, dst, "test-server", "test-store")
	must(t, e)
	defer restored.Close()
	a, e := restored.Read(ctx, k)
	must(t, e)
	if a != (Amount{50, 200}) {
		t.Fatal(a)
	}
	if _, e = Open(ctx, dir, "other", "test-store"); e == nil {
		t.Fatal("wrong identity")
	}
	if e = s.Reserve(ctx, k, "too much", "up", 751); !errors.Is(e, ErrBudget) {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, k, "overflow", "core2", "epoch2", "up", 0, math.MaxInt64, 1002, false); e == nil {
		t.Fatal("overflow")
	}
	if e = os.Chmod(dir, 0755); e != nil {
		t.Fatal(e)
	}
	if _, e = Open(ctx, dir, "test-server", "test-store"); e == nil {
		t.Fatal("permissions")
	}
	must(t, os.Chmod(dir, 0700))
	if _, e = Open(ctx, filepath.Join(t.TempDir(), "missing"), "test-server", "test-store"); e == nil {
		t.Fatal("missing store")
	}
}
func TestCrashAndParallel(t *testing.T) {
	if dir := os.Getenv("ROVU_SQLITE_TEST_DIR"); dir != "" {
		ctx := context.Background()
		s, e := Open(ctx, dir, "test-server", "test-store")
		must(t, e)
		k := Key{"alice", "p1"}
		switch os.Getenv("ROVU_SQLITE_ACTION") {
		case "mid":
			e = s.write(ctx, func(ctx context.Context, c *sql.Conn) error {
				_, x := c.ExecContext(ctx, "UPDATE periods SET limit_bytes=1 WHERE account='alice'")
				must(t, x)
				os.Exit(23)
				return nil
			})
		case "reserve":
			e = s.Reserve(ctx, k, "uncertain", "down", 10)
			must(t, e)
			os.Exit(24)
		case "settle":
			n, _ := strconv.ParseInt(os.Getenv("ROVU_SQLITE_N"), 10, 64)
			e = s.Settle(ctx, k, "T", n)
		case "restore":
			e = s.ImportOpening(ctx, cp(k))
		}
		must(t, e)
		s.Close()
		return
	}
	for _, n := range []int64{40, 100} {
		ctx, s, dir, k := newTestStore(t)
		must(t, s.Reserve(ctx, k, "T", "up", 100))
		child := func(action string) *exec.Cmd {
			c := quotaTestCommand("-test.run=^TestCrashAndParallel$")
			c.Env = append(os.Environ(), "ROVU_SQLITE_TEST_DIR="+dir, "ROVU_SQLITE_ACTION="+action, fmt.Sprintf("ROVU_SQLITE_N=%d", n))
			return c
		}
		for action, code := range map[string]int{"mid": 23, "reserve": 24} {
			e := child(action).Run()
			var x *exec.ExitError
			if !errors.As(e, &x) || x.ExitCode() != code {
				t.Fatal(action, e)
			}
		}
		a, b := child("settle"), child("restore")
		must(t, a.Start())
		must(t, b.Start())
		must(t, a.Wait())
		must(t, b.Wait())
		v, e := s.Read(ctx, k)
		must(t, e)
		if v != (Amount{200 + n, 10}) {
			t.Fatal(v)
		}
		if !errors.Is(s.Reserve(ctx, k, "uncertain", "down", 10), ErrReplay) {
			t.Fatal("lost crash receipt")
		}
	}
}
func TestBusy(t *testing.T) {
	ctx, s, dir, k := newTestStore(t)
	second, e := Open(ctx, dir, "test-server", "test-store")
	must(t, e)
	defer second.Close()
	c, e := s.db.Conn(ctx)
	must(t, e)
	defer c.Close()
	_, e = c.ExecContext(ctx, "BEGIN IMMEDIATE")
	must(t, e)
	start := time.Now()
	if e = second.Reserve(ctx, k, "blocked", "up", 2); e == nil || time.Since(start) > 4*time.Second {
		t.Fatal("expected bounded lock failure", e)
	}
	_, e = c.ExecContext(ctx, "ROLLBACK")
	must(t, e)
	c.Close()
	v, e := second.Read(ctx, k)
	must(t, e)
	if v != (Amount{}) {
		t.Fatal(v)
	}
}
