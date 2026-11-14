package handlers

import (
	"encoding/json"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/usage"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstallAccountingMixedServerRetry(t *testing.T) {
	c, _, p := unlimitedFixture(t)
	old := c.Users[0]
	fresh := newUser("newuser", "test", 0, 0, time.Now())
	limited := newUser("finite", "test", 0, 1, time.Now())
	unknown := newUser("unsampled", "test", 0, 0, time.Now())
	archived := newUser("archived", "test", 0, 0, time.Now())
	c.AddUser(fresh)
	c.AddUser(limited)
	c.AddUser(unknown)
	c.Archived = append(c.Archived, config.ArchivedUser{UserConfig: archived})
	ok16(t, c.Save())
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	before := aliases16(t, old.SubToken)
	// Simulate termination after publishing a database but before config selects it.
	_, e := installPeriod(c.Path(), c.ServerID, old, 200)
	ok16(t, e)
	ok16(t, os.WriteFile(p, []byte(`{"`+old.Username+`":250}`), 0600))
	for i := 0; i < 2; i++ {
		ok16(t, installAccounting(c, map[string]bool{fresh.AccountID: true}))
	}
	if c.Users[0].SubToken != old.SubToken || c.Users[0].PeriodID != old.PeriodID || c.Users[0].UsageBase != 30 || c.Users[2].SQLiteMeter != nil || c.Users[3].SQLiteMeter != nil || c.Archived[0].PeriodID != archived.PeriodID {
		t.Fatal("identity or policy changed")
	}
	for i, n := range []int64{220, 0} {
		m, _, e := boundAmount(c.ServerID, c.Users[i])
		ok16(t, e)
		if m.Confirmed != n {
			t.Fatal(m)
		}
	}
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	after := aliases16(t, old.SubToken)
	for i := range before {
		if i < 2 && string(before[i]) != string(after[i]) {
			t.Fatal("saved feed changed", i)
		}
	}
	counts, _ := json.Marshal(map[string]int64{old.Username: 350, fresh.Username: 40, unknown.Username: 65})
	ok16(t, os.WriteFile(p, counts, 0600))
	ok16(t, installAccounting(c, nil))
	ok16(t, ObserveUnlimitedAccounts(c))
	ok16(t, ObserveUnlimitedAccounts(c))
	for i, n := range map[int]int64{0: 320, 1: 40, 3: 65} {
		m, _, e := boundAmount(c.ServerID, c.Users[i])
		ok16(t, e)
		if m.Confirmed != n {
			t.Fatal(i, m)
		}
	}
}

func TestInstallAccountingRejectsBadCounters(t *testing.T) {
	c, _, p := unlimitedFixture(t)
	old := c.Users[0]
	for _, body := range []string{"{", `{"` + old.Username + `":1}`, `{"` + old.Username + `":-1}`} {
		ok16(t, os.WriteFile(p, []byte(body), 0600))
		if installAccounting(c, nil) == nil {
			t.Fatal("bad counter accepted")
		}
	}
	if c.Users[0].SQLiteMeter != nil || c.Users[0].SubToken != old.SubToken {
		t.Fatal("changed config on refusal")
	}
}
func TestInstallAccountingCandidateGate(t *testing.T) {
	c, _, _ := unlimitedFixture(t)
	ok16(t, installAccounting(c, nil))
	path := filepath.Join(t.TempDir(), "candidate")
	for _, test := range []struct {
		output string
		pass   bool
	}{{"old binary", false}, {OnlineUpdateABI, true}} {
		ok16(t, os.WriteFile(path, []byte("#!/bin/sh\necho '"+test.output+"'\n"), 0700))
		dir, e := prepareCandidateRecovery(c, path)
		if (e == nil) != test.pass {
			t.Fatal(e)
		}
		if test.pass {
			if _, e = os.Stat(filepath.Join(dir, "snapshot.sha256")); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestInstallAccountingAbsentSource(t *testing.T) {
	c, _, _ := unlimitedFixture(t)
	u := newUser("first", "test", 0, 0, time.Now())
	c.Users = []config.UserConfig{u}
	ok16(t, c.Save())
	usage.Dir = filepath.Join(t.TempDir(), "not-yet-created")
	ok16(t, rememberNewPeriod(c, u))
	ok16(t, installAccounting(c, nil))
	m, _, e := boundAmount(c.ServerID, c.Users[0])
	ok16(t, e)
	if m.Confirmed != 0 {
		t.Fatal(m)
	}
}
