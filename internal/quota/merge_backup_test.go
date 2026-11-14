package quota

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeBackupBothOrders(t *testing.T) {
	for _, n := range []int64{40, 100} {
		for _, first := range []bool{true, false} {
			ctx, s, _, k := newTestStore(t)
			must(t, s.Reserve(ctx, k, "T", "down", 100))
			branchDir := filepath.Join(t.TempDir(), "branch")
			must(t, s.Backup(ctx, branchDir))
			branch, e := Open(ctx, branchDir, "test-server", "test-store")
			must(t, e)
			must(t, branch.ImportOpening(ctx, cp(k)))
			must(t, branch.Settle(ctx, k, "T", n))
			snap := filepath.Join(t.TempDir(), "snapshot")
			must(t, branch.Backup(ctx, snap))
			must(t, branch.Close())
			raw, e := os.ReadFile(filepath.Join(snap, "ledger.sqlite"))
			must(t, e)
			digest := fmt.Sprintf("%x", sha256.Sum256(raw))
			if first {
				must(t, s.MergeBackup(ctx, k, snap, digest))
			}
			must(t, s.Settle(ctx, k, "T", n))
			must(t, s.MergeBackup(ctx, k, snap, digest))
			a, e := s.Read(ctx, k)
			must(t, e)
			if a != (Amount{200 + n, 0}) {
				t.Fatal(a)
			}
			if s.MergeBackup(context.Background(), k, snap, "wrong") == nil {
				t.Fatal("wrong hash accepted")
			}
		}
	}
}

// Reconciliation carries observations forward for unlimited/closed periods;
// it never reopens them, changes quota metadata or grants fresh traffic.
func TestMergeBackupUnlimitedAndClosedKeepsCurrentMetadata(t *testing.T) {
	for _, state := range []string{"active", "closed"} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			dir := filepath.Join(t.TempDir(), "live")
			s, e := Create(ctx, dir, "server", "store")
			must(t, e)
			defer s.Close()
			k := Key{"account", "period"}
			must(t, s.ImportUnlimited(ctx, k, "opening-operation", "proof", 300, 1000))
			must(t, s.FinishBootstrap(dir))
			must(t, s.ObserveUnlimited(ctx, k, 1050, 1))
			if state == "closed" {
				_, e = s.db.Exec("UPDATE periods SET status='closed' WHERE account=? AND period=?", k.Account, k.Period)
				must(t, e)
			}
			snap := filepath.Join(t.TempDir(), "snapshot")
			must(t, s.Backup(ctx, snap))
			raw, e := os.ReadFile(filepath.Join(snap, "ledger.sqlite"))
			must(t, e)
			digest := fmt.Sprintf("%x", sha256.Sum256(raw))
			if state == "active" {
				must(t, s.ObserveUnlimited(ctx, k, 1100, 2))
			}
			must(t, s.MergeBackup(ctx, k, snap, digest))
			must(t, s.MergeBackup(ctx, k, snap, digest))
			m, e := s.Metadata(ctx, k)
			must(t, e)
			want := int64(400)
			if state == "closed" {
				want = 350
			}
			if m.Confirmed != want || m.State != state || m.LimitKind != "unlimited" {
				t.Fatal(m)
			}
			if s.ReserveNative(ctx, k, "must-not-grant", "up", 1) == nil {
				t.Fatal("reconcile granted new transfer authority")
			}
			_, e = s.db.Exec("UPDATE periods SET limit_kind='finite',limit_bytes=1000 WHERE account=? AND period=?", k.Account, k.Period)
			must(t, e)
			// Use a new digest receipt key so prior successful replay cannot hide a
			// metadata mismatch; the actual new snapshot still describes unlimited.
			newer := filepath.Join(t.TempDir(), "new-snapshot")
			branch, e := Open(ctx, snap, "server", "store")
			must(t, e)
			_, e = branch.db.Exec("UPDATE periods SET sequence=sequence+1")
			must(t, e)
			must(t, branch.Backup(ctx, newer))
			must(t, branch.Close())
			raw, e = os.ReadFile(filepath.Join(newer, "ledger.sqlite"))
			must(t, e)
			if s.MergeBackup(ctx, k, newer, fmt.Sprintf("%x", sha256.Sum256(raw))) == nil {
				t.Fatal("different limit metadata accepted")
			}
		})
	}
}
