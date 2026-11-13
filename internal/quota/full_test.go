package quota

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDiskFullAndIntegrity(t *testing.T) {
	ctx, s, _, k := newTestStore(t)
	var pages int
	if e := s.db.QueryRow("PRAGMA page_count").Scan(&pages); e != nil {
		t.Fatal(e)
	}
	var bound int
	if e := s.db.QueryRow(fmt.Sprintf("PRAGMA max_page_count=%d", pages)).Scan(&bound); e != nil {
		t.Fatal(e)
	}
	var last Amount
	for i := 0; i < 5000; i++ {
		var e error
		last, e = s.Read(ctx, k)
		must(t, e)
		op := fmt.Sprintf("%d-%s", i, strings.Repeat("x", 150))
		e = s.Reserve(ctx, k, op, "up", 1)
		if e != nil {
			if errors.Is(e, ErrBudget) || !strings.Contains(strings.ToLower(e.Error()), "full") {
				t.Fatal("unexpected error", e)
			}
			v, re := s.Read(ctx, k)
			must(t, re)
			if v != last {
				t.Fatal("state changed on full")
			}
			var ok string
			must(t, s.db.QueryRow("PRAGMA integrity_check").Scan(&ok))
			if ok != "ok" {
				t.Fatal(ok)
			}
			t.Logf("SQLITE_FULL after %d commits under page limit %d; unchanged balance %+v", i, bound, v)
			return
		}
	}
	t.Fatal("did not hit disk full")
}
func TestCompatibleTicketCheckpoint(t *testing.T) {
	ctx, s, _, k := newTestStore(t)
	must(t, s.Reserve(ctx, k, "t", "up", 100))
	n := int64(40)
	snap := TicketSnapshot{"test-server", "test-store", "alice", "p1", "same", []TicketProof{{"t", "up", 100, &n}}}
	must(t, s.MergeTickets(ctx, snap))
	must(t, s.Settle(ctx, k, "t", 40))
	n = 100
	if e := s.MergeTickets(ctx, snap); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	a, e := s.Read(context.Background(), k)
	must(t, e)
	if a != (Amount{40, 0}) {
		t.Fatal(a)
	}
}
