package coreinstall

import (
	"context"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/usage"
	"os"
	"path/filepath"
	"testing"
)

func TestSampleQuotaEpochBinding(t *testing.T) {
	ctx := context.Background()
	d := t.TempDir()
	originalRoot, oldUsage, oldStats, oldEpoch := Root, usage.Dir, runStats, runEpoch
	t.Cleanup(func() { Root = originalRoot; usage.Dir = oldUsage; runStats = oldStats; runEpoch = oldEpoch })
	Root = filepath.Join(d, "cores")
	usage.Dir = filepath.Join(d, "usage")
	if e := os.MkdirAll(filepath.Join(Root, "one"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(Root, "one", stateFile), []byte(`{"apiPort":53421}`), 0600); e != nil {
		t.Fatal(e)
	}
	cfg := config.Default()
	cfg.ServerID = "server"
	cfg.Users = []config.UserConfig{{Username: "alice", AccountID: "a", PeriodID: "p"}}
	path := filepath.Join(d, "c")
	if e := cfg.SaveTo(path); e != nil {
		t.Fatal(e)
	}
	s, e := quota.Create(ctx, filepath.Join(d, "db"), "server", "store")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	k := quota.Key{Account: "a", Period: "p"}
	if e = s.NewPeriod(ctx, k, 5000); e != nil {
		t.Fatal(e)
	}
	runEpoch = func(string) (string, error) { return "epoch", nil }
	runStats = func(_ string, reset bool) ([]byte, error) {
		if reset {
			t.Fatal("counter reset")
		}
		return []byte(`{"stat":[{"name":"user>>>alice>>>traffic>>>uplink","value":110},{"name":"user>>>alice>>>traffic>>>downlink","value":220}]}`), nil
	}
	for i := 0; i < 2; i++ {
		if e = SamplePreviewQuota(ctx, s, path, "server", "one", "alice", "a", "p", "epoch", 100, 200); e != nil {
			t.Fatal(e)
		}
	}
	a, e := s.Read(ctx, k)
	if e != nil || a.Confirmed != 30 {
		t.Fatal(a, e)
	}
	runStats = func(_ string, reset bool) ([]byte, error) {
		if reset {
			t.Fatal("reset")
		}
		return []byte(`{"stat":[{"name":"user>>>alice>>>traffic>>>uplink","value":999}]}`), nil
	}
	if e = SamplePreviewQuota(ctx, s, path, "server", "one", "alice", "a", "p", "epoch", 100, 200); e == nil {
		t.Fatal("half sample accepted")
	}
	after, e := s.Read(ctx, k)
	if e != nil || after != a {
		t.Fatal("half sample changed ledger", after, e)
	}
	runEpoch = func(string) (string, error) { return "next", nil }
	if e = SamplePreviewQuota(ctx, s, path, "server", "one", "alice", "a", "p", "epoch", 100, 200); e == nil {
		t.Fatal("accepted changed epoch")
	}
}
