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
