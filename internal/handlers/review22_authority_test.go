package handlers

import (
	"context"
	"encoding/json"
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"

	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func revisionFixture22(t *testing.T) (*config.Config, StatusV2) {
	c, s, k := bound19(t)
	ctx := context.Background()
	_, e := ResolveV2FromConfig(c.Path(), nil, "")(ctx, c.Users[0].SubToken, "status.json")
	ok16(t, e)
	ok16(t, s.ReserveNative(ctx, k, "test22", "up", 41))
	ok16(t, s.Settle(ctx, k, "test22", 41))
	snap, e := ResolveV2FromConfig(c.Path(), nil, "")(ctx, c.Users[0].SubToken, "status.json")
	ok16(t, e)
	var v StatusV2
	ok16(t, json.Unmarshal(snap.Body, &v))
	if v.Revision <= 1 || v.Confirmed == nil || *v.Confirmed != 41 {
		t.Fatal("inadequate authority fixture")
	}
	c.SubscriptionV2 = true
	ok16(t, c.Save())
	return c, v
}
func TestRevisionAuthorityLossLifecycle22(t *testing.T) {
	for _, loss := range []string{"records", "anchor", "both", "truncated", "wrong-identity", "empty", "completion", "all"} {
		for _, action := range []string{"refresh", "serve-restart", "enable", "disable-enable", "post-update"} {
			t.Run(loss+"/"+action, func(t *testing.T) {
				c, v := revisionFixture22(t)
				root, anchor := authorityPaths(c)
				records := filepath.Join(root, "records.json")
				switch loss {
				case "records":
					ok16(t, os.Remove(records))
				case "anchor":
					ok16(t, os.Remove(anchor))
				case "both":
					ok16(t, os.Remove(records))
					ok16(t, os.Remove(anchor))
				case "completion":
					ok16(t, os.Remove(c.Path()+".status-v2.anchor"))
				case "all":
					ok16(t, os.Remove(records))
					ok16(t, os.Remove(anchor))
					ok16(t, os.Remove(c.Path()+".status-v2.anchor"))
				case "truncated":
					ok16(t, os.WriteFile(records, []byte("{"), 0600))
				case "wrong-identity":
					ok16(t, os.WriteFile(anchor, []byte("{}"), 0600))
				case "empty":
					ok16(t, os.WriteFile(records, []byte("{}"), 0600))
				}
				before := map[string]string{}
				for _, p := range []string{records, anchor, c.Path() + ".status-v2.anchor"} {
					b, e := os.ReadFile(p)
					if os.IsNotExist(e) {
						before[p] = "missing"
					} else {
						ok16(t, e)
						before[p] = digest(b)
					}
				}
				id := c.V2Authority.ID
				var e error
				switch action {
				case "refresh":
					e = ApplyV2Deployment(c)
				case "serve-restart":
					e = ServeSubscriptionV2(context.Background(), c.Path())
				case "enable":
					e = SetV2Deployment(c, true)
				case "disable-enable":
					c.SubscriptionV2 = false
					ok16(t, c.Save())
					e = SetV2Deployment(c, true)
				case "post-update":
					e = HandlePostUpdate(&actions.Context{Config: c, Output: discardOutput{}})
				}
				if e == nil {
					t.Fatal("lost authority accepted")
				}
				fresh, e := config.LoadFrom(c.Path())
				ok16(t, e)
				if fresh.V2Authority.ID != id || fresh.V2Authority.State != "ready" {
					t.Fatal("authority binding replaced")
				}
				for p, want := range before {
					b, e := os.ReadFile(p)
					got := digest(b)
					if os.IsNotExist(e) {
						got = "missing"
					}
					if got != want {
						t.Fatal("lifecycle reset private authority", p)
					}
				}
				recorder := httptest.NewRecorder()
				h := SubscriptionMetadataHandler(ResolveInstalledV2(c.Path()))
				h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v2/sub/"+c.Users[0].SubToken+".status.json", nil))
				want := 503
				if action == "disable-enable" {
					want = 404
				}
				if recorder.Code != want || recorder.Header().Get("Cache-Control") != "no-store" {
					t.Fatal(recorder.Code, recorder.Body.String(), v.Revision)
				}
			})
		}
	}
}
func TestRevisionAuthorityInitAndCommitRecovery22(t *testing.T) {
	if file := os.Getenv("RV22_REV_CONFIG"); file != "" {
		c, e := config.LoadFrom(file)
		ok16(t, e)
		phase := os.Getenv("RV22_REV_PHASE")
		revisionBarrier = func(s string) error {
			if s == phase {
				return syscall.Kill(os.Getpid(), syscall.SIGKILL)
			}
			return nil
		}
		if os.Getenv("RV22_REV_INIT") == "1" {
			ok16(t, InitializeV2Authority(c))
		} else {
			v := StatusV2{Server: c.ServerID, Account: c.Users[0].AccountID, Period: c.Users[0].PeriodID, Status: "active", Version: 2}
			n := int64(77)
			v.Confirmed = &n
			ok16(t, commitStatusRevision(c, &v))
		}
		return
	}
	for _, phase := range []string{"init-binding", "init-files", "init-ready", "revision-prepared", "revision-records", "revision-anchor"} {
		t.Run(phase, func(t *testing.T) {
			var c *config.Config
			init := phase[:4] == "init"
			if init {
				c = fixture16(t)
			} else {
				c, _ = revisionFixture22(t)
			}
			command := exec.Command(os.Args[0], "-test.run=^TestRevisionAuthorityInitAndCommitRecovery22$")
			command.Env = append(os.Environ(), "RV22_REV_CONFIG="+c.Path(), "RV22_REV_PHASE="+phase)
			if init {
				command.Env = append(command.Env, "RV22_REV_INIT=1")
			}
			err := command.Run()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
				t.Fatal(err)
			}
			ok16(t, c.Reload())
			id := c.V2Authority.ID
			if init {
				ok16(t, InitializeV2Authority(c))
			} else {
				if ValidateV2Authority(c) == nil {
					t.Fatal("pending commit accepted")
				}
				ok16(t, RecoverV2Revision(c))
			}
			if c.V2Authority.ID != id {
				t.Fatal("identity reset")
			}
			ok16(t, ValidateV2Authority(c))
		})
	}
}
func TestRevisionAuthorityCheckpoint22(t *testing.T) {
	c, v := revisionFixture22(t)
	root, anchor := authorityPaths(c)
	records := filepath.Join(root, "records.json")
	checkpoint := filepath.Join(t.TempDir(), "checkpoint")
	hash, e := CheckpointV2Authority(c, checkpoint)
	ok16(t, e)
	saved, e := os.ReadFile(records)
	ok16(t, e)
	ok16(t, os.Remove(records))
	ok16(t, os.Remove(anchor))
	if RepairV2Authority(c, checkpoint, "not-a-trusted-hash") == nil {
		t.Fatal("untrusted repair")
	}
	ok16(t, RepairV2Authority(c, checkpoint, hash))
	ok16(t, ValidateV2Authority(c))
	raw, e := os.ReadFile(records)
	ok16(t, e)
	if string(raw) != string(saved) {
		t.Fatal("checkpoint reset")
	}
	var registry revisionRegistry
	ok16(t, json.Unmarshal(raw, &registry))
	for _, r := range registry.Records {
		if r.Revision < v.Revision || r.Confirmed == nil || *r.Confirmed != 41 {
			t.Fatal("revision/balance regressed")
		}
	}
	// A newer surviving revision prevents repair from an older checkpoint.
	next := v
	next.Status = "hold"
	ok16(t, commitStatusRevision(c, &next))
	if RepairV2Authority(c, checkpoint, hash) == nil {
		t.Fatal("stale checkpoint replaced surviving high-water")
	}
}
