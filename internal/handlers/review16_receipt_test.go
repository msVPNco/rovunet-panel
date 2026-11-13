package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

func ok16(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func fixture16(t *testing.T) *config.Config {
	t.Helper()
	isolatedSubscriptions(t)
	useLocalPublicationTLS(t)
	c := subTestConfig()
	ok16(t, c.SaveTo(filepath.Join(t.TempDir(), "cfg")))
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	return c
}
func aliases16(t *testing.T, token string) [][]byte {
	t.Helper()
	var got [][]byte
	for _, suffix := range aliasSuffixes {
		b, e := os.ReadFile(filepath.Join(publicDir(), token+suffix))
		ok16(t, e)
		got = append(got, b)
	}
	return got
}
func absent16(t *testing.T, token string) {
	t.Helper()
	for _, s := range aliasSuffixes {
		if _, e := os.Stat(filepath.Join(publicDir(), token+s)); !os.IsNotExist(e) {
			t.Fatal("retired alias remained", s)
		}
	}
}
func same16(t *testing.T, before, after [][]byte) {
	t.Helper()
	for i := range before {
		if !bytes.Equal(before[i], after[i]) {
			t.Fatal("sibling content changed", i)
		}
	}
}
func child16(cfg *config.Config, user, id, phase string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestRotationReceiptRecovery$")
	cmd.Env = append(os.Environ(), "RV16_ROT_CFG="+cfg.Path(), "RV16_ROOT="+transport.SubscriptionDir, "RV16_USER="+user, "RV16_OP="+id, "RV16_KILL="+phase)
	return cmd
}
func TestRotationReceiptRecovery(t *testing.T) {
	if file := os.Getenv("RV16_ROT_CFG"); file != "" {
		transport.SubscriptionDir = os.Getenv("RV16_ROOT")
		useLocalPublicationTLS(t)
		c, e := config.LoadFrom(file)
		ok16(t, e)
		publicationBarrier = func(v string) error {
			if v == os.Getenv("RV16_KILL") {
				return syscall.Kill(os.Getpid(), syscall.SIGKILL)
			}
			return nil
		}
		ok16(t, RotateSubscriptionOperation(c, os.Getenv("RV16_USER"), os.Getenv("RV16_OP")))
		return
	}
	for _, phase := range []string{"before-render", "after-render", "before-exchange-bridge", "after-exchange-bridge", "after-fsync-bridge", "before-cas", "cas-before-file-sync", "cas-before-rename", "cas-after-rename", "cas-after-dir-sync", "before-exchange-final", "after-exchange-final", "after-fsync-final", "receipt-durable", "gc-bridge", "gc-final", "gc-owner", "stage-gc", "response-lost"} {
		t.Run(phase, func(t *testing.T) {
			cfg := fixture16(t)
			old := cfg.Users[0].SubToken
			bob := cfg.Users[1].SubToken
			before := aliases16(t, bob)
			op := config.NewID()
			foreign := filepath.Join(transport.SubscriptionDir, ".rotation-unrelated")
			ok16(t, os.Mkdir(foreign, 0700))
			e := child16(cfg, "alice", op, phase).Run()
			var x *exec.ExitError
			if !errors.As(e, &x) || x.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatalf("did not reach %s: %v", phase, e)
			}
			var intended string
			b, e := os.ReadFile(journalPath())
			if e == nil {
				var j rotateIntent
				ok16(t, json.Unmarshal(b, &j))
				intended = j.New
			} else {
				if !os.IsNotExist(e) {
					t.Fatal(e)
				}
				ok16(t, cfg.Reload())
				intended = cfg.Users[0].SubToken
			}
			for i := 0; i < 2; i++ {
				ok16(t, child16(cfg, "alice", op, "").Run())
				ok16(t, cfg.Reload())
				if cfg.Users[0].SubToken != intended || intended == old {
					t.Fatal("minted a second token")
				}
			}
			r, ok, e := getReceipt(op)
			ok16(t, e)
			if !ok || r.Outcome != "done" || r.TokenDigest != digest([]byte(intended)) {
				t.Fatal("missing stable result")
			}
			list, e := os.ReadDir(receiptDir())
			ok16(t, e)
			if len(list) != 1 {
				t.Fatal("unexpected receipt count")
			}
			list, e = os.ReadDir(publicDir())
			ok16(t, e)
			if len(list) != 8 {
				t.Fatal("unexpected public objects", len(list))
			}
			for _, p := range []string{journalPath(), filepath.Join(transport.SubscriptionDir, ".rotation-"+cfg.ServerID+"-"+op)} {
				if _, e = os.Stat(p); !os.IsNotExist(e) {
					t.Fatal("owned GC incomplete")
				}
			}
			if _, e = os.Stat(foreign); e != nil {
				t.Fatal("foreign stage removed")
			}
			absent16(t, old)
			aliases16(t, intended)
			same16(t, before, aliases16(t, bob))
		})
	}
}
func TestRotationConcurrentRequestIdentity(t *testing.T) {
	c := fixture16(t)
	op := config.NewID()
	a, b := child16(c, "alice", op, ""), child16(c, "alice", op, "")
	ok16(t, a.Start())
	ok16(t, b.Start())
	ok16(t, a.Wait())
	ok16(t, b.Wait())
	ok16(t, c.Reload())
	first := c.Users[0].SubToken
	if e := RotateSubscriptionOperation(c, "bob", op); e == nil {
		t.Fatal("request identity collision")
	}
	ok16(t, RotateSubscriptionOperation(c, "alice", op))
	if c.Users[0].SubToken != first {
		t.Fatal("duplicate minted")
	}
	ok16(t, RotateSubscriptionOperation(c, "alice", ""))
	defaultToken := c.Users[0].SubToken
	ok16(t, RotateSubscriptionOperation(c, "alice", ""))
	if defaultToken == first || c.Users[0].SubToken != defaultToken {
		t.Fatal("blank ID retry is not stable")
	}
	if e := RotateSubscriptionOperation(c, "alice", op); !errors.Is(e, errSuperseded) {
		t.Fatal("obsolete request returned another operation's capability")
	}
	list, e := os.ReadDir(receiptDir())
	ok16(t, e)
	if len(list) != 2 {
		t.Fatal("unexpected receipt count")
	}
}
func TestRotationSecuritySupersedes(t *testing.T) {
	for _, which := range []string{"password", "revoke", "expiry", "renew", "sqlite-limit", "sqlite-unavailable"} {
		t.Run(which, func(t *testing.T) {
			c := fixture16(t)
			op := config.NewID()
			old := c.Users[0].SubToken
			bob := c.Users[1].SubToken
			before := aliases16(t, bob)
			stop := errors.New("pause")
			publicationBarrier = func(v string) error {
				if v == "bridge" {
					return stop
				}
				return nil
			}
			e := RotateSubscriptionOperation(c, "alice", op)
			publicationBarrier = nil
			if !errors.Is(e, stop) {
				t.Fatal(e)
			}
			var j rotateIntent
			b, e := os.ReadFile(journalPath())
			ok16(t, e)
			ok16(t, json.Unmarshal(b, &j))
			u := c.Users[0]
			switch which {
			case "password":
				u.Password = "changed-password"
				u.SubToken = config.NewSubToken()
			case "renew":
				u.PeriodID = config.NewID()
				u.SubToken = config.NewSubToken()
			case "expiry":
				u.ExpiresAt = time.Now().Add(-time.Second).UTC().Format(time.RFC3339)
			case "sqlite-limit", "sqlite-unavailable":
				dir := filepath.Join(t.TempDir(), "s")
				ctx := context.Background()
				s, e := quota.Create(ctx, dir, c.ServerID, "s")
				ok16(t, e)
				defer s.Close()
				k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
				ok16(t, s.NewPeriod(ctx, k, 1<<30))
				ok16(t, s.FinishBootstrap(dir))
				ok16(t, s.Reserve(ctx, k, "T", "up", 1<<30))
				ok16(t, s.Settle(ctx, k, "T", 1<<30))
				u.QuotaGB = 1
				u.SQLiteMeter = &config.SQLiteBinding{Directory: dir, Store: "s"}
				if which == "sqlite-unavailable" {
					u.SQLiteMeter.Store = "wrong"
				}
			}
			if which == "revoke" {
				c.RemoveUser("alice")
			} else {
				c.AddUser(u)
			}
			ok16(t, c.Save())
			for i := 0; i < 2; i++ {
				if e := RotateSubscriptionOperation(c, "alice", op); !errors.Is(e, errSuperseded) {
					t.Fatal("did not persist supersede", e)
				}
			}
			r, ok, e := getReceipt(op)
			ok16(t, e)
			if !ok || r.Outcome != "superseded" {
				t.Fatal("missing tombstone")
			}
			absent16(t, old)
			absent16(t, j.New)
			same16(t, before, aliases16(t, bob))
			ok16(t, WriteSubscriptions(c, discardOutput{}))
			for _, tk := range []string{old, j.New} {
				for _, suffix := range aliasSuffixes[:2] {
					if _, e := os.Stat(filepath.Join(publicDir(), tk+suffix)); !os.IsNotExist(e) {
						t.Fatal("stale feed resurrected")
					}
				}
			}
			if which == "password" {
				payload := aliases16(t, u.SubToken)[0]
				if len(payload) == 0 {
					t.Fatal("new credential not published")
				}
			}
		})
	}
}

func TestRotationLedgerChangeWithoutConfigWrite(t *testing.T) {
	c := fixture16(t)
	ctx := context.Background()
	u := c.Users[0]
	// A real finite native feed allows the first render to succeed.
	secret := filepath.Join(t.TempDir(), "secret")
	ok16(t, os.WriteFile(secret, []byte(strings.Repeat("17", 32)), 0600))
	c.Tunnels = append(c.Tunnels, config.TunnelConfig{Tag: "vless", Enabled: true, Transport: config.TransportVLESS, Domain: "fixture.test", VLESS: &config.VLESSConfig{Port: 443, Path: "/ws", SecretFile: secret, TLS: true, CDN: true}})
	dir := filepath.Join(t.TempDir(), "ledger")
	s, e := quota.Create(ctx, dir, c.ServerID, "s")
	ok16(t, e)
	defer s.Close()
	k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
	ok16(t, s.NewPeriod(ctx, k, 1<<30))
	ok16(t, s.FinishBootstrap(dir))
	u.SQLiteMeter = &config.SQLiteBinding{Directory: dir, Store: "s"}
	u.QuotaGB = 1
	c.AddUser(u)
	ok16(t, c.Save())
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	id := config.NewID()
	hold := errors.New("hold")
	publicationBarrier = func(p string) error {
		if p == "bridge" {
			return hold
		}
		return nil
	}
	e = RotateSubscriptionOperation(c, "alice", id)
	publicationBarrier = nil
	if !errors.Is(e, hold) {
		t.Fatal(e)
	}
	before, e := os.ReadFile(c.Path())
	ok16(t, e)
	ok16(t, s.Reserve(ctx, k, "last", "up", 1<<30))
	ok16(t, s.Settle(ctx, k, "last", 1<<30))
	var j rotateIntent
	b, e := os.ReadFile(journalPath())
	ok16(t, e)
	ok16(t, json.Unmarshal(b, &j))
	if !errors.Is(RotateSubscriptionOperation(c, "alice", id), errSuperseded) {
		t.Fatal("ledger did not supersede")
	}
	after, e := os.ReadFile(c.Path())
	ok16(t, e)
	if !bytes.Equal(before, after) {
		t.Fatal("test changed config")
	}
	absent16(t, j.Old)
	absent16(t, j.New)
}

func TestRotationBootOffline(t *testing.T) {
	for _, phase := range []string{"after-exchange-bridge", "cas-after-rename", "receipt-durable"} {
		t.Run(phase, func(t *testing.T) {
			c := fixture16(t)
			id := config.NewID()
			old := c.Users[0].SubToken
			err := child16(c, "alice", id, phase).Run()
			var x *exec.ExitError
			if !errors.As(err, &x) || x.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatal(err)
			}
			var j rotateIntent
			b, e := os.ReadFile(journalPath())
			ok16(t, e)
			ok16(t, json.Unmarshal(b, &j))
			probe := probePublished
			probePublished = func(string, string) error { t.Fatal("boot attempted to contact offline server"); return nil }
			ok16(t, RecoverBeforeServing(c))
			probePublished = probe
			ok16(t, c.Reload())
			aliases16(t, c.Users[0].SubToken)
			aliases16(t, c.Users[1].SubToken)
			if c.Users[0].SubToken == old {
				absent16(t, j.New)
			} else {
				absent16(t, old)
			}
			ok16(t, RotateSubscriptionOperation(c, "alice", id))
			if c.Users[0].SubToken != j.New {
				t.Fatal("boot minted another capability")
			}
			absent16(t, old)
		})
	}
}
