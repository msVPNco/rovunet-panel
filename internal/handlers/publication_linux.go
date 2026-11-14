package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"time"
)

var aliasSuffixes = []string{".json", ".windows.json", ".status.json", ".windows.status.json", trustSuffix}

type rotateIntent struct{ Server, User, Account, Period, Old, New, ConfigHash, OldHash, BridgeHash, FinalHash, Stage, Operation, Request, SelectedHash string }

var publicationBarrier func(string) error // package-local scheduling control used by fault tests
func barrier(point string) error {
	if publicationBarrier != nil {
		return publicationBarrier(point)
	}
	return nil
}
func digest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func journalPath() string    { return filepath.Join(transport.SubscriptionDir, ".rotation.json") }
func tokenOK(t string) bool {
	b, e := hex.DecodeString(t)
	return e == nil && len(b) == 16 && hex.EncodeToString(b) == t
}
func publicDir() string { return filepath.Join(transport.SubscriptionDir, "sub") }
func treeDigest(dir string) (string, error) {
	files, e := os.ReadDir(dir)
	if e != nil {
		return "", e
	}
	h := sha256.New()
	for _, f := range files {
		if !f.Type().IsRegular() {
			return "", errors.New("non-regular publication entry")
		}
		data, e := os.ReadFile(filepath.Join(dir, f.Name()))
		if e != nil {
			return "", e
		}
		fmt.Fprintf(h, "%d:%s:%s\n", len(f.Name()), f.Name(), digest(data))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func copyGeneration(from, to string) error {
	if e := os.Mkdir(to, 0711); e != nil {
		return e
	}
	entries, e := os.ReadDir(from)
	if e != nil {
		return e
	}
	for _, f := range entries {
		if !f.Type().IsRegular() {
			return errors.New("unexpected subscription entry")
		}
		data, e := os.ReadFile(filepath.Join(from, f.Name()))
		if e != nil {
			return e
		}
		if e = safefile.Write(filepath.Join(to, f.Name()), data, 0644); e != nil {
			return e
		}
	}
	return safefile.SyncDir(to)
}
func swapTo(public, stage, wantCurrent, wantNext string) error {
	current, e := treeDigest(public)
	if e != nil {
		return e
	}
	if current == wantNext {
		return syncGenerationParents(public, stage)
	}
	if current != wantCurrent {
		return errors.New("publication generation conflict")
	}
	next, e := treeDigest(stage)
	if e != nil || next != wantNext {
		return errors.New("invalid staged publication")
	}
	phase := filepath.Base(stage)
	if e = barrier("before-exchange-" + phase); e != nil {
		return e
	}
	if e = unix.Renameat2(unix.AT_FDCWD, public, unix.AT_FDCWD, stage, unix.RENAME_EXCHANGE); e != nil {
		return e
	}
	if e = barrier("after-exchange-" + phase); e != nil {
		return e
	}
	if e = syncGenerationParents(public, stage); e != nil {
		return e
	}
	return barrier("after-fsync-" + phase)
}
func syncGenerationParents(a, b string) error {
	return errors.Join(safefile.SyncDir(filepath.Dir(a)), safefile.SyncDir(filepath.Dir(b)))
}
func beginRotation(cfg *config.Config, user, id string) error {
	u := cfg.GetUser(user)
	if u == nil || !tokenOK(u.SubToken) || !validPublicationID(u.AccountID, u.PeriodID, cfg.ServerID) {
		return errors.New("a published account with stable identifiers is required")
	}
	if e := uniqueSubscriptionOwner(cfg, u.SubToken, u.AccountID); e != nil {
		return e
	}
	raw, e := os.ReadFile(cfg.Path())
	if e != nil {
		return e
	}
	j := rotateIntent{User: user, Server: cfg.ServerID, Account: u.AccountID, Period: u.PeriodID, Old: u.SubToken, New: config.NewSubToken(), ConfigHash: digest(raw)}
	if j.Old == j.New {
		return errors.New("token collision")
	}
	j.Operation = id
	j.Request = rotationRequest(cfg.ServerID, user, u.AccountID, u.PeriodID, id)
	j.Stage = rotationStage(j)
	if _, err := os.Lstat(filepath.Join(transport.SubscriptionDir, j.Stage)); !os.IsNotExist(err) {
		return errors.New("rotation stage already exists and is unclaimed")
	}
	if !rotationEligible(cfg, *u) {
		return errors.New("rotation requires confirmed current access")
	}
	if e = saveIntent(j); e != nil {
		return e
	}
	_, e = resumeRotation(cfg)
	return e
}
func saveIntent(j rotateIntent) error {
	b, e := json.Marshal(j)
	if e != nil {
		return e
	}
	return safefile.Write(journalPath(), b, 0600)
}
func renderIntent(cfg *config.Config, u *config.UserConfig, j rotateIntent) error {
	stage := filepath.Join(transport.SubscriptionDir, j.Stage)
	if e := removeOwnedStage(j); e != nil {
		return e
	}
	if e := os.Mkdir(stage, 0700); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join(stage, "owner"), []byte(j.Request), 0600); e != nil {
		return e
	}
	f, e := os.Open(filepath.Join(stage, "owner"))
	if e != nil {
		return e
	}
	e = f.Sync()
	f.Close()
	if e != nil {
		return e
	}
	if e = safefile.SyncDir(stage); e != nil {
		return e
	}
	if e = barrier("before-render"); e != nil {
		return e
	}
	bridge := filepath.Join(stage, "bridge")
	final := filepath.Join(stage, "final")
	if j.OldHash, e = treeDigest(publicDir()); e != nil {
		return e
	}
	if e = copyGeneration(publicDir(), bridge); e != nil {
		return e
	}
	for _, v := range cfg.Users {
		if v.SubToken == j.New {
			return errors.New("token collision")
		}
	}
	for _, v := range cfg.Archived {
		if v.SubToken == j.New {
			return errors.New("archived token collision")
		}
	}
	for _, suffix := range aliasSuffixes {
		if _, e = os.Lstat(filepath.Join(bridge, j.New+suffix)); !os.IsNotExist(e) {
			return errors.New("token file collision")
		}
	}
	in := newStatusInput(cfg, time.Now())
	st := buildStatus(in, *u, "")
	if st.Status != statusActive {
		return errors.New("rotation requires an active account")
	}
	status, e := statusBody(st, filepath.Join(stage, "unused"))
	if e != nil {
		return e
	}
	for i, suffix := range aliasSuffixes {
		body := status
		if i < 2 {
			client := ""
			if i == 1 {
				client = "windows"
			}
			body, e = buildFeedForClient(cfg, *u, client)
			if e != nil {
				return e
			}
			var feed subFeed
			if json.Unmarshal(body, &feed) != nil || len(feed.Items) == 0 {
				return errors.New("no valid replacement feed")
			}
		}
		if suffix == trustSuffix {
			body, e = buildSubscriptionTrust(cfg, *u)
			if e != nil {
				return e
			}
		}
		if e = safefile.Write(filepath.Join(bridge, j.New+suffix), body, 0644); e != nil {
			return e
		}
	}
	if e = copyGeneration(bridge, final); e != nil {
		return e
	}
	for _, suffix := range aliasSuffixes {
		if e = os.Remove(filepath.Join(final, j.Old+suffix)); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	if e = safefile.SyncDir(final); e != nil {
		return e
	}
	if j.BridgeHash, e = treeDigest(bridge); e != nil {
		return e
	}
	if j.FinalHash, e = treeDigest(final); e != nil {
		return e
	}
	if e = safefile.SyncDir(stage); e != nil {
		return e
	}
	if e = barrier("after-render"); e != nil {
		return e
	}
	// Compare the *selected* snapshot on resume; the old bytes may be
	// legitimate but not canonically formatted (e.g. restored JSON).
	projected, e := config.LoadFrom(cfg.Path())
	if e != nil {
		return e
	}
	hash, e := projected.WithSnapshotDigest()
	if e != nil || fmt.Sprintf("%x", hash) != j.ConfigHash {
		return config.ErrConcurrentChange
	}
	v := *projected.GetUser(j.User)
	v.SubToken = j.New
	projected.AddUser(v)
	raw, e := json.MarshalIndent(projected, "", "  ")
	if e != nil {
		return e
	}
	j.SelectedHash = digest(append(raw, '\n'))
	if e = saveIntent(j); e != nil {
		return e
	}
	return barrier("prepared")
}
func validPublicationID(s ...string) bool {
	for _, v := range s {
		if !tokenOK(v) {
			return false
		}
	}
	return true
}
func resumeRotation(cfg *config.Config) (string, error) {
	data, e := os.ReadFile(journalPath())
	if os.IsNotExist(e) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	var j rotateIntent
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&j); e != nil || !json.Valid(data) || !validIntent(j) {
		return "", errors.New("invalid publication journal")
	}
	if e = cfg.RefreshIfChanged(); e != nil {
		return "", e
	}
	u := cfg.GetUser(j.User)
	stage := filepath.Join(transport.SubscriptionDir, j.Stage)
	if cfg.ServerID != j.Server {
		return "", errors.New("journal belongs to another installation")
	}
	if r, ok, e := getReceipt(j.Operation); e != nil {
		return "", e
	} else if ok {
		if r != receiptFor(j, r.Outcome) {
			return "", errors.New("receipt disagrees with intent")
		}
		if e = cleanRotation(j); e != nil {
			return "", e
		}
		if r.Outcome == "superseded" {
			return "", errSuperseded
		}
		return j.User, nil
	}
	if u == nil || cfg.ServerID != j.Server || u.AccountID != j.Account || u.PeriodID != j.Period || !rotationEligible(cfg, *u) || (u.SubToken != j.Old && u.SubToken != j.New) {
		return "", cfg.WithCurrentFile(func() error { return abandonRotation(cfg, j, stage) })
	}
	bridge := filepath.Join(stage, "bridge")
	final := filepath.Join(stage, "final")
	if u.SubToken == j.Old {
		raw, e := os.ReadFile(cfg.Path())
		if e != nil {
			return "", e
		}
		if digest(raw) != j.ConfigHash {
			return "", cfg.WithCurrentFile(func() error { return abandonRotation(cfg, j, stage) })
		}
		if j.FinalHash == "" {
			if e = renderIntent(cfg, u, j); e != nil {
				return "", e
			}
			return resumeRotation(cfg)
		}
		if e = cfg.WithCurrentFile(func() error { return swapTo(publicDir(), bridge, j.OldHash, j.BridgeHash) }); e != nil {
			return "", e
		}
		if e = barrier("bridge"); e != nil {
			return "", e
		}
		// Read back all bytes from the serving tree before selecting the token.
		got, e := treeDigest(publicDir())
		if e != nil || got != j.BridgeHash {
			return "", errors.New("public verification failed")
		}
		if e = probePublished(subscriptionHost(cfg), j.New); e != nil {
			return "", e
		}
		next := *u
		next.SubToken = j.New
		cfg.AddUser(next)
		if e = barrier("before-cas"); e != nil {
			return "", e
		}
		if e = cfg.SaveObserved(func(point string) error { return barrier("cas-" + point) }); e != nil {
			_ = cfg.Reload()
			return "", e
		}
		if e = barrier("selected"); e != nil {
			return "", e
		}
	}
	// If rename completed but its directory fsync did not, make the new
	// config name durable before retiring the old capability or issuing a receipt.
	if e = safefile.SyncDir(filepath.Dir(cfg.Path())); e != nil {
		return "", e
	}
	selected, e := os.ReadFile(cfg.Path())
	if e != nil {
		return "", e
	}
	if digest(selected) != j.SelectedHash {
		return "", cfg.WithCurrentFile(func() error { return abandonRotation(cfg, j, stage) })
	}
	if e = cfg.WithCurrentFile(func() error {
		if !rotationEligible(cfg, *u) {
			return abandonRotation(cfg, j, stage)
		}
		if err := swapTo(publicDir(), final, j.BridgeHash, j.FinalHash); err != nil {
			return err
		}
		if err := barrier("retired"); err != nil {
			return err
		}
		return finishRotation(j, "done")
	}); e != nil {
		return "", e
	}
	return j.User, nil
}
func finishRotation(j rotateIntent, outcome string) error {
	if e := storeReceipt(j, outcome); e != nil {
		return e
	}
	if e := barrier("receipt-durable"); e != nil {
		return e
	}
	return cleanRotation(j)
}
func cleanRotation(j rotateIntent) error {
	if e := removeOwnedStage(j); e != nil {
		return e
	}
	if e := safefile.SyncDir(transport.SubscriptionDir); e != nil {
		return e
	}
	if e := barrier("stage-gc"); e != nil {
		return e
	}
	if e := os.Remove(journalPath()); e != nil {
		return e
	}
	if e := safefile.SyncDir(transport.SubscriptionDir); e != nil {
		return e
	}
	return barrier("response-lost")
}
func abandonRotation(cfg *config.Config, j rotateIntent, stage string) error {
	// Stale intents may not republish a revoked token. Remove the capabilities of
	// the obsolete operation unless the fresh config still binds them, then let
	// the ordinary writer refresh all authorized accounts.
	keep := map[string]bool{}
	for _, a := range cfg.Archived {
		if a.AccountID != j.Account {
			keep[a.SubToken] = true
		}
	}
	for _, u := range cfg.Users {
		if u.AccountID != j.Account || (u.PeriodID == j.Period && rotationEligible(cfg, u)) {
			keep[u.SubToken] = true
		}
	}
	for _, t := range []string{j.Old, j.New} {
		if keep[t] {
			continue
		}
		for _, suffix := range aliasSuffixes {
			e := os.Remove(filepath.Join(publicDir(), t+suffix))
			if e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	if e := safefile.SyncDir(publicDir()); e != nil {
		return e
	}
	if e := finishRotation(j, "superseded"); e != nil {
		return e
	}
	return errSuperseded
}

func uniqueSubscriptionOwner(cfg *config.Config, token, account string) error {
	count := 0
	for _, u := range cfg.Users {
		if u.SubToken == token {
			if u.AccountID != account {
				return errors.New("ambiguous token binding")
			}
			count++
		}
	}
	for _, a := range cfg.Archived {
		if a.SubToken == token && a.AccountID != account {
			return errors.New("ambiguous archived token binding")
		}
	}
	if count != 1 {
		return errors.New("ambiguous token binding")
	}
	return nil
}
