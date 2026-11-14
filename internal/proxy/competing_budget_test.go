package proxy

import (
	"context"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"path/filepath"
	"testing"
	"time"
)

// Independent native listeners model distinct bridge workers sharing the same
// durable period. Concurrent streams must never multiply the remaining budget.
func TestCompetingListenersShareFiniteBudget(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "db")
	st, e := quota.Create(ctx, dir, "server", "store")
	need16(t, e)
	defer st.Close()
	key := quota.Key{Account: "A", Period: "p1"}
	need16(t, st.NewPeriod(ctx, key, 1<<30))
	need16(t, st.ImportOpening(ctx, quota.Checkpoint{Server: "server", Store: "store", Account: "A", Period: "p1", ID: "opening", OpeningOrigin: "fixture", Proof: "synthetic", Opening: (1 << 30) - 1000}))
	need16(t, st.FinishBootstrap(dir))
	cfg := &config.Config{ServerID: "server", Users: []config.UserConfig{{Username: "alice", Password: "pw-alice", AccountID: "A", PeriodID: "p1", QuotaGB: 1, SQLiteMeter: &config.SQLiteBinding{Directory: dir, Store: "store"}}, {Username: "bob", Password: "pw-bob", AccountID: "B", PeriodID: "p1"}}}
	host, port := startEchoTCP(t)
	clients := []*testWSClient{}
	for i := 0; i < 2; i++ {
		env := startVLESS(t, false, true, func(o *VLESSOptions) { need16(t, cfg.SaveTo(o.UsersFile)); o.QuotaStore = st; o.QuotaServer = "server" })
		c, _ := dialWS(t, env, testWSPath, "")
		need16(t, c.writeFrame(2, buildVLESSRequest(env.userID("alice", "pw-alice"), 1, host, port)))
		expectResponseHeader(t, c)
		defer c.conn.Close()
		clients = append(clients, c)
	}
	start := make(chan struct{})
	done := make(chan int, 2)
	for _, c := range clients {
		go func(c *testWSClient) {
			<-start
			_ = c.conn.SetDeadline(time.Now().Add(4 * time.Second))
			n := 0
			for i := 0; i < 20; i++ {
				if c.writeFrame(2, make([]byte, 50)) != nil {
					break
				}
				_, b, e := c.readFrame()
				n += len(b)
				if e != nil {
					break
				}
			}
			done <- n
		}(c)
	}
	close(start)
	received := <-done + <-done
	deadline := time.Now().Add(3 * time.Second)
	var amount quota.Amount
	for {
		amount, e = st.Read(ctx, key)
		need16(t, e)
		if amount.Pending == 0 && amount.Confirmed == 1<<30 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("shared budget did not settle", amount)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if received <= 0 || received > 1000 {
		t.Fatal("invalid aggregate payload", received)
	}
	t.Logf("two independent listeners: confirmed exactly limit, pending=0, overshoot=0, received=%d", received)
}
