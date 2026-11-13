package quota

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestKilledSettlement(t *testing.T) {
	if dir := os.Getenv("ROVU_SQL_KILL"); dir != "" {
		ctx := context.Background()
		s, e := Open(ctx, dir, "test-server", "test-store")
		must(t, e)
		mode := os.Getenv("ROVU_SQL_KILL_POINT")
		if mode == "after" {
			must(t, s.Settle(ctx, Key{"alice", "p1"}, "T", 40))
		} else {
			_ = s.write(ctx, func(ctx context.Context, c *sql.Conn) error {
				_, e := c.ExecContext(ctx, "UPDATE tickets SET actual=40 WHERE op='T'")
				must(t, e)
				return syscall.Kill(os.Getpid(), syscall.SIGKILL)
			})
		}
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		return
	}
	for _, mode := range []string{"before", "after"} {
		ctx, s, dir, k := newTestStore(t)
		must(t, s.Reserve(ctx, k, "T", "up", 100))
		cmd := exec.Command(os.Args[0], "-test.run=^TestKilledSettlement$")
		cmd.Env = append(os.Environ(), "ROVU_SQL_KILL="+dir, "ROVU_SQL_KILL_POINT="+mode)
		var x *exec.ExitError
		if e := cmd.Run(); !errors.As(e, &x) || x.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			t.Fatal(e)
		}
		actual, e := s.Read(ctx, k)
		must(t, e)
		want := Amount{0, 100}
		if mode == "after" {
			want = Amount{40, 0}
		}
		if actual != want {
			t.Fatalf("%s got %+v", mode, actual)
		}
		must(t, s.Settle(ctx, k, "T", 40))
	}
}
func TestCorruptionNeverRecreates(t *testing.T) {
	ctx, s, dir, _ := newTestStore(t)
	must(t, s.Close())
	p := filepath.Join(dir, "ledger.sqlite")
	b := []byte("broken sqlite state")
	must(t, os.WriteFile(p, b, 0600))
	if _, e := Open(ctx, dir, "test-server", "test-store"); e == nil {
		t.Fatal("accepted corruption")
	}
	current, e := os.ReadFile(p)
	must(t, e)
	if string(current) != string(b) {
		t.Fatal("overwrote damaged store")
	}
}
