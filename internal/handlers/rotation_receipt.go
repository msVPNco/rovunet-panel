package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"os"
	"path/filepath"
)

var errSuperseded = errors.New("rotation superseded; no replacement capability issued by this retry")

type rotationReceipt struct{ Operation, Request, Server, Account, Period, User, Outcome, TokenDigest string }

func rotationRequest(server, user, account, period, id string) string {
	b, _ := json.Marshal([]string{"rotate-link-v1", server, user, account, period, id})
	return digest(b)
}
func receiptDir() string                  { return filepath.Join(transport.SubscriptionDir, ".rotation-receipts") }
func receiptPath(id string) string        { return filepath.Join(receiptDir(), id+".json") }
func rotationStage(j rotateIntent) string { return ".rotation-" + j.Server + "-" + j.Operation }
func receiptFor(j rotateIntent, outcome string) rotationReceipt {
	return rotationReceipt{j.Operation, j.Request, j.Server, j.Account, j.Period, j.User, outcome, digest([]byte(j.New))}
}
func getReceipt(id string) (rotationReceipt, bool, error) {
	var r rotationReceipt
	b, e := os.ReadFile(receiptPath(id))
	if os.IsNotExist(e) {
		return r, false, nil
	}
	if e != nil {
		return r, false, e
	}
	if e = json.Unmarshal(b, &r); e != nil || r.Operation != id || !validPublicationID(r.Server, r.Account, r.Period, id) || r.Request != rotationRequest(r.Server, r.User, r.Account, r.Period, id) || (r.Outcome != "done" && r.Outcome != "superseded") || len(r.TokenDigest) != 64 {
		return r, false, errors.New("invalid rotation receipt")
	}
	return r, true, nil
}
func storeReceipt(j rotateIntent, outcome string) error {
	r := receiptFor(j, outcome)
	if old, ok, e := getReceipt(j.Operation); e != nil {
		return e
	} else if ok {
		if old != r {
			return errors.New("conflicting operation receipt")
		}
		return nil
	}
	if e := os.Mkdir(receiptDir(), 0700); e != nil && !os.IsExist(e) {
		return e
	}
	if e := safefile.SyncDir(transport.SubscriptionDir); e != nil {
		return e
	}
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return safefile.Write(receiptPath(j.Operation), b, 0600)
}
func checkReceipt(cfg *config.Config, user, id string) (bool, error) {
	r, ok, e := getReceipt(id)
	if !ok || e != nil {
		return false, e
	}
	if cfg.ServerID != r.Server || user != r.User {
		return true, errors.New("operation ID was used for a different request")
	}
	u := cfg.GetUser(user)
	if r.Outcome != "done" || u == nil || u.AccountID != r.Account || u.PeriodID != r.Period || digest([]byte(u.SubToken)) != r.TokenDigest || !rotationEligible(cfg, *u) {
		return true, errSuperseded
	}
	return true, nil
}

// Only a stage explicitly claimed by this installation and this intent may
// be removed. Never sweep the subscription root by filename prefix.
func removeOwnedStage(j rotateIntent) error {
	stage := filepath.Join(transport.SubscriptionDir, j.Stage)
	if j.Stage != rotationStage(j) {
		return errors.New("stage ownership mismatch")
	}
	st, e := os.Lstat(stage)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return errors.New("invalid private stage")
	}
	b, e := os.ReadFile(filepath.Join(stage, "owner"))
	if os.IsNotExist(e) {
		entries, err := os.ReadDir(stage)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return os.Remove(stage)
		}
	}
	if e != nil || !bytes.Equal(b, []byte(j.Request)) {
		return errors.New("stage belongs to a different operation")
	}
	entries, e := os.ReadDir(stage)
	if e != nil {
		return e
	}
	for _, ent := range entries {
		switch ent.Name() {
		case "owner":
			continue
		case "bridge", "final":
			if !ent.IsDir() {
				return errors.New("unexpected stage object")
			}
		default:
			return errors.New("unowned stage object")
		}
	}
	for _, name := range []string{"bridge", "final"} {
		if e = os.RemoveAll(filepath.Join(stage, name)); e != nil {
			return e
		}
		if e = barrier("gc-" + name); e != nil {
			return e
		}
	}
	if e = safefile.SyncDir(stage); e != nil {
		return e
	}
	if e = os.Remove(filepath.Join(stage, "owner")); e != nil {
		return e
	}
	if e = barrier("gc-owner"); e != nil {
		return e
	}
	return os.Remove(stage)
}
