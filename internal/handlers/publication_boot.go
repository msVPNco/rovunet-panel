package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"os"
	"path/filepath"
)

// Caddy is not running in ExecStartPre, so an online HTTPS probe would
// deadlock boot. Only the config-selected aliases may be exposed at boot.
// The CLI can later resume a not-yet-selected request and do the live probe.
func RecoverBeforeServing(cfg *config.Config) error {
	return withPublicationLock(func() error {
		if e := cfg.RefreshIfChanged(); e != nil {
			return e
		}
		if e := resumeCarry(cfg); e != nil {
			return e
		}
		b, e := os.ReadFile(journalPath())
		if os.IsNotExist(e) {
			return writeCurrentSubscriptions(cfg, &actions.StdOutput{})
		}
		if e != nil {
			return e
		}
		var j rotateIntent
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if e = dec.Decode(&j); e != nil || !json.Valid(b) || !validIntent(j) || cfg.ServerID != j.Server {
			return errors.New("unrecognized rotation; refusing to serve")
		}
		u := cfg.GetUser(j.User)
		if u != nil && u.SubToken == j.Old && u.AccountID == j.Account && u.PeriodID == j.Period && rotationEligible(cfg, *u) {
			raw, e := os.ReadFile(cfg.Path())
			if e != nil {
				return e
			}
			if digest(raw) == j.ConfigHash {
				// Rebuild from current authority. Retain the original operation/token
				// and invalidate old staged hashes before dirtying the public generation.
				j.OldHash = ""
				j.BridgeHash = ""
				j.FinalHash = ""
				j.SelectedHash = ""
				if e = saveIntent(j); e != nil {
					return e
				}
				return writeCurrentSubscriptions(cfg, &actions.StdOutput{})
			}
		}
		// After selection, the online probe was already successful. Complete using
		// the recorded hashes; outdated authority instead retires its aliases.
		_, e = resumeRotation(cfg)
		if e != nil && !errors.Is(e, errSuperseded) {
			return e
		}
		return writeCurrentSubscriptions(cfg, &actions.StdOutput{})
	})
}
func validIntent(j rotateIntent) bool {
	return j.Old != j.New && validPublicationID(j.Old, j.New, j.Operation, j.Account, j.Period, j.Server) && filepath.Base(j.Stage) == j.Stage && j.Stage == rotationStage(j) && j.Request == rotationRequest(j.Server, j.User, j.Account, j.Period, j.Operation)
}
