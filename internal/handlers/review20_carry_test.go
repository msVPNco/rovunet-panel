package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/recoveryauth"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

func plan20(t *testing.T, c *config.Config, s *quota.Store, k quota.Key) (CarryPlan, []byte, string) {
	t.Helper()
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	ok16(t, e)
	raw, e := os.ReadFile(c.Path())
	ok16(t, e)
	a, e := s.Read(context.Background(), k)
	ok16(t, e)
	u := c.GetUser("alice")
	plan := CarryPlan{Version: 1, Operation: config.NewID(), Server: c.ServerID, User: "alice", Account: u.AccountID, OldPeriod: u.PeriodID, NewPeriod: config.NewID(), Store: u.SQLiteMeter.Store, Directory: u.SQLiteMeter.Directory, ConfigHash: digest(raw), Limit: u.QuotaBytes(), Confirmed: a.Confirmed, Pending: a.Pending, ExpiresAt: u.ExpiresAt}
	signed, e := recoveryauth.Sign(plan, key)
	ok16(t, e)
	return plan, signed, hex.EncodeToString(pub)
}
func TestCarryCoordinatorRecovery(t *testing.T) {
	if file := os.Getenv("RV20_CARRY_CONFIG"); file != "" {
		transport.SubscriptionDir = os.Getenv("RV20_CARRY_ROOT")
		useLocalPublicationTLS(t)
		c, e := config.LoadFrom(file)
		ok16(t, e)
		signed, e := os.ReadFile(os.Getenv("RV20_CARRY_PLAN"))
		ok16(t, e)
		carryBarrier = func(p string) error {
			if p == os.Getenv("RV20_CARRY_PHASE") {
				return syscall.Kill(os.Getpid(), syscall.SIGKILL)
			}
			return nil
		}
		ok16(t, CarryAccountPeriod(c, signed, os.Getenv("RV20_CARRY_PUB")))
		return
	}
	for _, phase := range []string{"prepared", "ledger-closed", "config-selected", "published", "receipt"} {
		t.Run(phase, func(t *testing.T) {
			c, s, k := bound19(t)
			ctx := context.Background()
			u := c.Users[0]
			u.ExpiresAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
			c.AddUser(u)
			ok16(t, c.Save())
			ok16(t, s.ReserveNative(ctx, k, "T", "up", 40))
			ok16(t, s.Settle(ctx, k, "T", 40))
			plan, signed, pub := plan20(t, c, s, k)
			file := filepath.Join(t.TempDir(), "plan")
			ok16(t, os.WriteFile(file, signed, 0600))
			cmd := exec.Command(os.Args[0], "-test.run=^TestCarryCoordinatorRecovery$")
			cmd.Env = append(os.Environ(), "RV20_CARRY_CONFIG="+c.Path(), "RV20_CARRY_ROOT="+transport.SubscriptionDir, "RV20_CARRY_PLAN="+file, "RV20_CARRY_PUB="+pub, "RV20_CARRY_PHASE="+phase)
			err := cmd.Run()
			var killed *exec.ExitError
			if !errors.As(err, &killed) || killed.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatal(err)
			}
			// Resume through the ordinary publisher, as a timer/boot writer would.
			ok16(t, WriteSubscriptions(c, discardOutput{}))
			ok16(t, CarryAccountPeriod(c, signed, pub))
			ok16(t, c.Reload())
			next := c.GetUser("alice")
			if next.PeriodID != plan.NewPeriod || next.SubToken != u.SubToken || next.ExpiresAt != u.ExpiresAt {
				t.Fatal("identity/link/expiry mismatch")
			}
			amount, e := s.Read(ctx, quota.Key{Account: k.Account, Period: plan.NewPeriod})
			ok16(t, e)
			if amount != (quota.Amount{Confirmed: 40}) {
				t.Fatal(amount)
			}
			if s.ReserveNative(ctx, k, "old", "up", 1) == nil {
				t.Fatal("old budget open")
			}
			aliases16(t, next.SubToken)
			aliases16(t, c.GetUser("bob").SubToken)
			if _, e = os.Stat(carryPending(c)); !os.IsNotExist(e) {
				t.Fatal("pending not retired")
			}
		})
	}
}
func TestCarryAuthenticationAndUnknown(t *testing.T) {
	c, s, k := bound19(t)
	ctx := context.Background()
	plan, b, pub := plan20(t, c, s, k)
	b[len(b)-3] ^= 1
	if CarryAccountPeriod(c, b, pub) == nil {
		t.Fatal("unauthenticated carry")
	}
	_, b, pub = plan20(t, c, s, k)
	ok16(t, s.ReserveNative(ctx, k, "pending", "up", 100))
	if CarryAccountPeriod(c, b, pub) == nil {
		t.Fatal("unknown pending transferred")
	}
	if _, e := s.Read(ctx, quota.Key{Account: k.Account, Period: plan.NewPeriod}); e == nil {
		t.Fatal("new budget appeared")
	}
}
func TestSealedRecoveryTarget(t *testing.T) {
	c, s, k := bound19(t)
	ctx := context.Background()
	ok16(t, PrepareV2Authority(c))
	c.SubscriptionV2 = true
	ok16(t, c.Save())
	ok16(t, RotateSubscriptionOperation(c, "alice", config.NewID()))
	ok16(t, s.ReserveNative(ctx, k, "T", "up", 40))
	ok16(t, s.Settle(ctx, k, "T", 40))
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	op := config.NewID()
	hash, e := SealRecoverySnapshot(ctx, c, snapshot, op)
	ok16(t, e)
	again, e := SealRecoverySnapshot(ctx, c, snapshot, op)
	ok16(t, e)
	if hash != again {
		t.Fatal("seal replay changed")
	}
	if !c.RecoverySealed {
		t.Fatal("source not frozen")
	}
	target := filepath.Join(t.TempDir(), "fresh")
	id, e := InitializeRecoveryTarget(target)
	ok16(t, e)
	pub, key, e := ed25519.GenerateKey(rand.Reader)
	ok16(t, e)
	pinned := hex.EncodeToString(pub)
	claim := RecoveryClaim{Version: 1, Sequence: 2, Status: "claimed", Instance: c.InstanceID, Snapshot: hash, Operation: config.NewID(), Target: id, SealOperation: op}
	envelope, e := recoveryauth.Sign(claim, key)
	ok16(t, e)
	ok16(t, RestoreSealedTarget(ctx, target, snapshot, envelope, pinned))
	ok16(t, RestoreSealedTarget(ctx, target, snapshot, envelope, pinned))
	fresh, e := config.LoadFrom(filepath.Join(target, "state/config.json"))
	ok16(t, e)
	oldSecret, e := os.ReadFile(c.Tunnels[len(c.Tunnels)-1].VLESS.SecretFile)
	ok16(t, e)
	restoredSecret, e := os.ReadFile(fresh.Tunnels[len(fresh.Tunnels)-1].VLESS.SecretFile)
	ok16(t, e)
	if string(oldSecret) != string(restoredSecret) {
		t.Fatal("native credential root changed")
	}
	if fresh.RecoverySealed || fresh.InstanceID == c.InstanceID {
		t.Fatal("target not exclusive")
	}
	u := fresh.GetUser("alice")
	restored, e := quota.Open(ctx, u.SQLiteMeter.Directory, fresh.ServerID, u.SQLiteMeter.Store)
	ok16(t, e)
	defer restored.Close()
	a, e := restored.Read(ctx, k)
	ok16(t, e)
	if a.Confirmed != 40 {
		t.Fatal(a)
	}
	ok16(t, restored.ReserveNative(ctx, k, "after-recovery", "up", 1))
	ok16(t, restored.Settle(ctx, k, "after-recovery", 1))
	ok16(t, RestoreSealedTarget(ctx, target, snapshot, envelope, pinned))
	a, e = restored.Read(ctx, k)
	ok16(t, e)
	if a.Confirmed != 41 {
		t.Fatal("recovery rolled back")
	}
	wrong := filepath.Join(t.TempDir(), "other")
	_, e = InitializeRecoveryTarget(wrong)
	ok16(t, e)
	if RestoreSealedTarget(ctx, wrong, snapshot, envelope, pinned) == nil {
		t.Fatal("claimed budget restored twice")
	}
	if !c.RecoverySealed {
		t.Fatal("old installation unsealed")
	}
}
