package proxy

import (
	"context"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"path/filepath"
	"testing"
	"time"
)

func TestFiniteAndUnlimitedShareListenerWithoutBudgetBypass(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "db")
	st, e := quota.Create(ctx, dir, "server", "store")
	need16(t, e)
	defer st.Close()
	k := quota.Key{Account: "A", Period: "p1"}
	need16(t, st.NewPeriod(ctx, k, 1<<30))
	need16(t, st.ImportOpening(ctx, quota.Checkpoint{Server: "server", Store: "store", Account: "A", Period: "p1", ID: "opening", OpeningOrigin: "fixture", Proof: "synthetic disjoint opening", Opening: (1 << 30) - 100}))
	need16(t, st.FinishBootstrap(dir))
	cfg := &config.Config{ServerID: "server", Users: []config.UserConfig{{Username: "alice", Password: "pw-alice", AccountID: "A", PeriodID: "p1", QuotaGB: 1, SQLiteMeter: &config.SQLiteBinding{Directory: dir, Store: "store"}}, {Username: "bob", Password: "pw-bob", AccountID: "B", PeriodID: "p1"}}}
	env := startVLESS(t, false, true, func(o *VLESSOptions) {
		need16(t, cfg.SaveTo(o.UsersFile))
		o.QuotaStore = st
		o.QuotaServer = "server"
		o.QuotaUsed = func(string) int64 { return 1<<63 - 1 }
	})
	host, port := startEchoTCP(t)
	open := func(user string) *testWSClient {
		c, _ := dialWS(t, env, testWSPath, "")
		need16(t, c.writeFrame(2, buildVLESSRequest(env.userID(user, "pw-"+user), 1, host, port)))
		expectResponseHeader(t, c)
		return c
	}
	bob := open("bob")
	defer bob.conn.Close()
	alice := open("alice")
	defer alice.conn.Close()
	need16(t, alice.writeFrame(2, make([]byte, 50)))
	// Exactly 50 upload + 50 download bytes exhaust the final 100-byte budget.
	received := readUntil(t, alice, nil, 50)
	if len(received) != 50 {
		t.Fatal("payload mismatch")
	}
	need16(t, alice.conn.SetReadDeadline(time.Now().Add(time.Second)))
	_, _, e = alice.readFrame()
	if e == nil {
		t.Fatal("exhausted connection stayed open")
	}
	amount, e := st.Read(ctx, k)
	need16(t, e)
	if amount.Confirmed != 1<<30 || amount.Pending != 0 {
		t.Fatal(amount)
	}
	need16(t, bob.writeFrame(2, []byte("still-open")))
	if string(readUntil(t, bob, nil, 10)) != "still-open" {
		t.Fatal("unlimited peer interrupted")
	}
	fresh := open("bob")
	fresh.conn.Close()
}
