package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/recoveryauth"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryAuthorityLossFailsClosed(t *testing.T) {
	for _, loss := range []string{"database", "receipts", "revisions"} {
		t.Run(loss, func(t *testing.T) {
			c, _, _ := bound19(t)
			ctx := context.Background()
			ok16(t, PrepareV2Authority(c))
			c.SubscriptionV2 = true
			ok16(t, c.Save())
			ok16(t, RotateSubscriptionOperation(c, "alice", config.NewID()))
			snapshot := filepath.Join(t.TempDir(), "snapshot")
			op := config.NewID()
			hash, e := SealRecoverySnapshot(ctx, c, snapshot, op)
			ok16(t, e)
			target := filepath.Join(t.TempDir(), "target")
			id, e := InitializeRecoveryTarget(target)
			ok16(t, e)
			pub, key, e := ed25519.GenerateKey(rand.Reader)
			ok16(t, e)
			pinned := hex.EncodeToString(pub)
			claim := RecoveryClaim{Version: 1, Sequence: 2, Status: "claimed", Instance: c.InstanceID, Snapshot: hash, Operation: config.NewID(), Target: id, SealOperation: op}
			envelope, e := recoveryauth.Sign(claim, key)
			ok16(t, e)
			ok16(t, RestoreSealedTarget(ctx, target, snapshot, envelope, pinned))
			path := filepath.Join(target, "state/config.json")
			fresh, e := config.LoadFrom(path)
			ok16(t, e)
			var missing string
			switch loss {
			case "database":
				missing = filepath.Join(fresh.Users[0].SQLiteMeter.Directory, "ledger.sqlite")
			case "receipts":
				dir := filepath.Join(target, "state/subscriptions/.rotation-receipts")
				entries, e := os.ReadDir(dir)
				ok16(t, e)
				for _, entry := range entries {
					if filepath.Ext(entry.Name()) == ".json" {
						missing = filepath.Join(dir, entry.Name())
						break
					}
				}
				if missing == "" {
					t.Fatal("receipt fixture missing")
				}
			case "revisions":
				missing = filepath.Join(path+".status-v2", "records.json")
				ok16(t, os.Remove(path+".status-v2.anchor"))
			}
			ok16(t, os.Remove(missing))
			if RestoreSealedTarget(ctx, target, snapshot, envelope, pinned) == nil {
				t.Fatal("lost authority silently reset")
			}
			if _, e = os.Stat(missing); !os.IsNotExist(e) {
				t.Fatal("authority was recreated")
			}
		})
	}
}
