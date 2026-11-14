package quota

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// A private tmpfs is filled by the lab runner for an actual storage-full test.
func TestFilesystemFull(t *testing.T) {
	root := os.Getenv("ROVU_SQL_FULL_ROOT")
	if root == "" {
		t.Skip("requires bounded private filesystem")
	}
	ctx := context.Background()
	d := filepath.Join(root, "db")
	s, e := Create(ctx, d, "test-server", "test-store")
	must(t, e)
	defer s.Close()
	k := Key{"alice", "p"}
	must(t, s.NewPeriod(ctx, k, 100000))
	must(t, s.FinishBootstrap(d))
	f, e := os.OpenFile(filepath.Join(root, "filler"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	must(t, e)
	defer f.Close()
	chunk := make([]byte, 64<<10)
	for {
		_, e = f.Write(chunk)
		if e != nil {
			break
		}
	}
	if !errors.Is(e, syscall.ENOSPC) {
		t.Fatal(e)
	}
	var count int64
	var failed error
	for i := 0; i < 5000; i++ {
		failed = s.Reserve(ctx, k, stringID(i), "up", 1)
		if failed != nil {
			break
		}
		count++
	}
	if failed == nil || !(strings.Contains(strings.ToLower(failed.Error()), "full") || strings.Contains(strings.ToLower(failed.Error()), "disk i/o")) {
		t.Fatal("expected storage failure", failed)
	}
	must(t, f.Truncate(0))
	v, e := s.Read(ctx, k)
	must(t, e)
	if v.Pending != count || v.Confirmed != 0 {
		t.Fatal(v, count)
	}
	t.Logf("private tmpfs ENOSPC, committed tickets=%d, denied transaction=%v", count, failed)
}
func stringID(i int) string { return fmt.Sprintf("reserve-%d", i) }
