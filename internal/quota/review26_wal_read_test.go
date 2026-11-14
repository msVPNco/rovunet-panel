package quota

import (
	"context"
	"testing"
	"time"
)

// Holding the only writer connection must not block policy reads. WAL readers
// see the last commit; they grant no traffic. Reserve still rechecks the period
// under the serialized write transaction after a pending freeze commits.
func TestWALPolicyReadsDuringWriter26(t *testing.T) {
	ctx, s, _, k := newTestStore(t)
	c, err := s.db.Conn(ctx)
	must(t, err)
	defer c.Close()
	_, err = c.ExecContext(ctx, "BEGIN IMMEDIATE")
	must(t, err)
	defer c.ExecContext(context.Background(), "ROLLBACK")
	_, err = c.ExecContext(ctx, "UPDATE periods SET status='frozen' WHERE account=? AND period=?", k.Account, k.Period)
	must(t, err)
	read, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	limit, err := s.Limit(read, k)
	must(t, err)
	if limit != 1000 {
		t.Fatal(limit)
	}
	a, err := s.Read(read, k)
	must(t, err)
	if a != (Amount{}) {
		t.Fatal(a)
	}
	m, err := s.Metadata(read, k)
	must(t, err)
	if m.State != "active" || m.Limit != 1000 {
		t.Fatal(m)
	}
	_, err = c.ExecContext(ctx, "COMMIT")
	must(t, err)
	must(t, c.Close())
	if err = s.ReserveNative(ctx, k, "after-freeze", "up", 1); err == nil {
		t.Fatal("a stale policy read granted traffic after the freeze committed")
	}
	a, err = s.Read(ctx, k)
	must(t, err)
	if a != (Amount{}) {
		t.Fatal("failed reserve changed usage", a)
	}
}

func TestWALPolicyPoolCannotWrite26(t *testing.T) {
	ctx, s, _, k := newTestStore(t)
	c, err := s.policyDB.Conn(ctx)
	must(t, err)
	defer c.Close()
	var enabled int
	must(t, c.QueryRowContext(ctx, "PRAGMA query_only").Scan(&enabled))
	if enabled != 1 {
		t.Fatal("policy connection is not query-only")
	}
	// Even accidental removal of the second protection cannot turn a mode=ro
	// SQLite connection into a writer.
	_, err = c.ExecContext(ctx, "PRAGMA query_only=OFF")
	must(t, err)
	if _, err = c.ExecContext(ctx, "UPDATE periods SET limit_bytes=2000 WHERE account=? AND period=?", k.Account, k.Period); err == nil {
		t.Fatal("read-only pool changed the account limit")
	}
	_, err = c.ExecContext(ctx, "PRAGMA query_only=ON")
	must(t, err)
	limit, err := s.Limit(context.Background(), k)
	must(t, err)
	if limit != 1000 {
		t.Fatal(limit)
	}
}
