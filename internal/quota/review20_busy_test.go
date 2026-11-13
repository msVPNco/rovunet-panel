package quota

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBusyBeforeBeginPreservesBudget(t *testing.T) {
	ctx, s, dir, k := newTestStore(t)
	second, e := Open(ctx, dir, "test-server", "test-store")
	must(t, e)
	defer second.Close()
	holder, e := s.db.Conn(ctx)
	must(t, e)
	defer holder.Close()
	_, e = holder.ExecContext(ctx, "BEGIN IMMEDIATE")
	must(t, e)
	result := make(chan error, 1)
	go func() { result <- second.Renew(ctx, k, "renew-busy", "p2", 1000) }()
	select {
	case e := <-result:
		t.Fatalf("did not wait for writer: %v", e)
	case <-time.After(400 * time.Millisecond):
	}
	_, e = holder.ExecContext(ctx, "COMMIT")
	must(t, e)
	must(t, <-result)
	must(t, second.Renew(ctx, k, "renew-busy", "p2", 1000))
	value, e := second.Read(ctx, Key{k.Account, "p2"})
	must(t, e)
	if value != (Amount{}) {
		t.Fatal(value)
	}
	if s.ReserveNative(ctx, k, "stale", "up", 1) == nil {
		t.Fatal("old budget open")
	}
	// A permanently held writer exits within the caller's deadline, no period.
	_, e = holder.ExecContext(ctx, "BEGIN IMMEDIATE")
	must(t, e)
	limited, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if second.Renew(limited, Key{k.Account, "p2"}, "blocked", "p3", 1000) == nil {
		t.Fatal("writer lock ignored")
	}
	_, e = holder.ExecContext(ctx, "ROLLBACK")
	must(t, e)
	if _, e = second.Read(ctx, Key{k.Account, "p3"}); e == nil {
		t.Fatal("failed begin created budget")
	}
	if e = waitBusy(ctx, func() error { return ErrConflict }); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
}
