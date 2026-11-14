package quota

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestMutationContextAndNoReplay22(t *testing.T) {
	ctx, s, _, k := newTestStore(t)
	called := 0
	fault := errors.New("fault after mutation")
	e := s.change(ctx, k, "test-deadline", "one", 0, func(txctx context.Context, c *sql.Conn) error {
		called++
		deadline, ok := txctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second || time.Until(deadline) <= 0 {
			t.Fatal("mutation did not receive bounded transaction context")
		}
		if _, e := c.ExecContext(txctx, "INSERT INTO opening VALUES(?,?,?,?,?)", k.Account, k.Period, "deadline-test", make([]byte, 32), 1); e != nil {
			return e
		}
		return fault
	})
	if !errors.Is(e, fault) || called != 1 {
		t.Fatal("mutation body replayed", called, e)
	}
	value, e := s.Read(ctx, k)
	must(t, e)
	if value != (Amount{}) {
		t.Fatal("failed mutation consumed budget", value)
	}
}
