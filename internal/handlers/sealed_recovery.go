package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/recoveryauth"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

type RecoveryClaim struct {
	SealOperation string `json:"seal_operation"`
	Version       int    `json:"version"`
	Sequence      int64  `json:"sequence"`
	Status        string `json:"status"`
	Instance      string `json:"instance"`
	Snapshot      string `json:"snapshot"`
	Operation     string `json:"operation"`
	Target        string `json:"target"`
}
type targetIdentity struct{ ID string }

func SealRecoverySnapshot(ctx context.Context, cfg *config.Config, dest, operation string) (string, error) {
	if cfg.V2Authority != nil || cfg.SubscriptionV2 {
		if e := ValidateV2Authority(cfg); e != nil {
			return "", e
		}
	}
	if !tokenOK(operation) {
		return "", errors.New("invalid seal operation")
	}
	if cfg.RecoverySealed && cfg.RecoverySealOperation != operation {
		return "", errors.New("another recovery seal is already active")
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = config.NewID()
	}
	cfg.RecoverySealOperation = operation
	cfg.RecoverySealed = true
	if e := cfg.Save(); e != nil {
		return "", e
	}
	if e := WriteSubscriptions(cfg, discardOutput{}); e != nil {
		return "", e
	}
	// A failed export retains the seal; there is no automatic unsafe rollback.
	if b, e := os.ReadFile(filepath.Join(dest, "manifest.json")); e == nil {
		var m CoherentManifest
		if json.Unmarshal(b, &m) != nil || m.Operation != operation || m.Server != cfg.ServerID {
			return "", errors.New("existing snapshot is not this seal")
		}
		raw, e := os.ReadFile(cfg.Path())
		if e != nil || digest(raw) != m.ConfigHash {
			return "", errors.New("sealed source configuration changed")
		}
		files, e := privateSnapshotFiles(dest)
		if e != nil {
			return "", e
		}
		a, _ := json.Marshal(files)
		x, _ := json.Marshal(m.Files)
		if string(a) != string(x) {
			return "", errors.New("existing snapshot integrity mismatch")
		}
		return digest(b), nil
	}
	return CreateCoherentSnapshot(ctx, cfg, dest, operation)
}
func InitializeRecoveryTarget(dir string) (string, error) {
	if e := os.Mkdir(dir, 0700); e != nil {
		return "", e
	}
	id := config.NewID()
	b, e := json.Marshal(targetIdentity{id})
	if e != nil {
		return "", e
	}
	if e = safefile.Write(filepath.Join(dir, "target.json"), b, 0600); e != nil {
		return "", e
	}
	if e = safefile.SyncDir(filepath.Dir(dir)); e != nil {
		return "", e
	}
	return id, nil
}
func ReleaseRecoverySeal(cfg *config.Config, envelope []byte, pinned string) error {
	var c RecoveryClaim
	if e := recoveryauth.Verify(envelope, pinned, &c); e != nil {
		return e
	}
	if c.Version != 1 || c.Status != "released" || c.Instance != cfg.InstanceID || c.SealOperation != cfg.RecoverySealOperation {
		return errors.New("release authority does not match sealed installation")
	}
	// Operator custody has durably invalidated the checkpoint before this save.
	if cfg.RecoverySealed {
		cfg.RecoverySealed = false
		if e := cfg.Save(); e != nil {
			return e
		}
	}
	return WriteSubscriptions(cfg, discardOutput{})
}

// RestoreSealedTarget is an offline fresh-target recovery. The externally
// authenticated custodian claim is bound to a newly generated target nonce.
// It never replaces a live installation, resets a missing DB, or accepts a
// previously consumed checkpoint for a second physical target.
func RestoreSealedTarget(ctx context.Context, dir, snapshot string, envelope []byte, pinned string) error {
	var c RecoveryClaim
	if e := recoveryauth.Verify(envelope, pinned, &c); e != nil {
		return e
	}
	if c.Version != 1 || c.Status != "claimed" || c.Sequence <= 0 || !validPublicationID(c.Target, c.Instance, c.Operation) {
		return errors.New("invalid current recovery claim")
	}
	identity, e := os.ReadFile(filepath.Join(dir, "target.json"))
	if e != nil {
		return e
	}
	var target targetIdentity
	if json.Unmarshal(identity, &target) != nil || target.ID != c.Target {
		return errors.New("claim belongs to another fresh target")
	}
	marker := filepath.Join(dir, "restore.receipt")
	if b, e := os.ReadFile(marker); e == nil {
		if string(b) != digest(envelope) {
			return errors.New("different recovery already committed")
		}
		return finishRestoredTarget(filepath.Join(dir, "state", "config.json"), c.Target)
	} else if !os.IsNotExist(e) {
		return e
	}
	raw, e := os.ReadFile(filepath.Join(snapshot, "manifest.json"))
	if e != nil {
		return e
	}
	if digest(raw) != c.Snapshot {
		return errors.New("not the externally approved latest checkpoint")
	}
	var m CoherentManifest
	if json.Unmarshal(raw, &m) != nil || m.Version != 1 {
		return errors.New("invalid snapshot manifest")
	}
	files, e := privateSnapshotFiles(snapshot)
	if e != nil {
		return e
	}
	a, _ := json.Marshal(files)
	b, _ := json.Marshal(m.Files)
	if string(a) != string(b) {
		return errors.New("snapshot integrity mismatch")
	}
	original, e := config.LoadFrom(filepath.Join(snapshot, "config.json"))
	if e != nil {
		return e
	}
	if !original.RecoverySealed || original.InstanceID != c.Instance || original.ServerID != m.Server {
		return errors.New("checkpoint was not sealed by the claimed source")
	}
	// State is assembled privately; existing unrelated objects are never swept.
	stage := filepath.Join(dir, "state")
	if _, e = os.Stat(stage); e == nil {
		own, e := os.ReadFile(filepath.Join(stage, "owner"))
		if e != nil || string(own) != digest(envelope) {
			return errors.New("unowned recovery stage")
		}
	} else if !os.IsNotExist(e) {
		return e
	} else {
		if e = os.Mkdir(stage, 0700); e != nil {
			return e
		}
		if e = safefile.Write(filepath.Join(stage, "owner"), []byte(digest(envelope)), 0600); e != nil {
			return e
		}
	}
	for i := range original.Tunnels {
		t := &original.Tunnels[i]
		name, ok := m.NativeSecrets[t.Tag]
		if !ok {
			continue
		}
		if t.VLESS == nil || filepath.Base(name) != name {
			return errors.New("invalid native secret mapping")
		}
		b, e := os.ReadFile(filepath.Join(snapshot, "runtime", name))
		if e != nil {
			return e
		}
		destination := filepath.Join(stage, "runtime", name)
		if e = os.MkdirAll(filepath.Dir(destination), 0700); e != nil {
			return e
		}
		if e = safefile.Write(destination, b, 0600); e != nil {
			return e
		}
		t.VLESS.SecretFile = destination
	}
	for _, db := range m.Databases {
		if filepath.Base(db.Snapshot) != db.Snapshot || !strings.HasPrefix(db.Snapshot, "db-") || db.Server != m.Server {
			return errors.New("invalid checkpoint database")
		}
		next := filepath.Join(stage, db.Snapshot)
		if e = copyRecoveryDB(snapshot, db.Snapshot, next, m.Files[db.Snapshot+"/ledger.sqlite"]); e != nil {
			return e
		}
		for i := range original.Users {
			u := &original.Users[i]
			if u.SQLiteMeter != nil && u.SQLiteMeter.Directory == db.Directory && u.SQLiteMeter.Store == db.Store {
				u.SQLiteMeter = &config.SQLiteBinding{Directory: next, Store: db.Store}
			}
		}
		for i := range original.Archived {
			u := &original.Archived[i]
			if u.SQLiteMeter != nil && u.SQLiteMeter.Directory == db.Directory && u.SQLiteMeter.Store == db.Store {
				u.SQLiteMeter = &config.SQLiteBinding{Directory: next, Store: db.Store}
			}
		}
	}
	for _, pair := range [][2]string{{"public", "subscriptions/sub"}, {"receipts", "subscriptions/.rotation-receipts"}, {"status-revisions", "config.json.status-v2"}, {"carry-operations", "config.json.carry"}, {"revision-adoption", "config.json.status-v2.adoption"}} {
		from := filepath.Join(snapshot, pair[0])
		if _, e = os.Stat(from); os.IsNotExist(e) {
			continue
		} else if e != nil {
			return e
		}
		to := filepath.Join(stage, pair[1])
		if e = os.MkdirAll(filepath.Dir(to), 0700); e != nil {
			return e
		}
		if _, e = os.Stat(to); os.IsNotExist(e) {
			if pair[0] == "public" {
				e = copyGeneration(from, to)
			} else {
				e = copyPrivateTree(from, to)
			}
			if e != nil {
				return e
			}
			if e = os.Chmod(to, 0700); e != nil {
				return e
			}
		}
	}
	if anchor, e := os.ReadFile(filepath.Join(snapshot, "status-revisions.anchor")); e == nil {
		if e = safefile.Write(filepath.Join(stage, "config.json.status-v2.anchor"), anchor, 0600); e != nil {
			return e
		}
	}
	if e = safefile.Write(filepath.Join(stage, "recovery-manifest.json"), raw, 0600); e != nil {
		return e
	}
	original.SubscriptionV2 = false // public routes stay opt-in on a fresh physical target
	original.InstanceID = target.ID
	original.RecoverySealed = true
	path := filepath.Join(stage, "config.json")
	// A resumed attempt must not overwrite consumption on a committed target.
	if e = original.SaveTo(path); e != nil {
		return e
	}
	if e = safefile.Write(marker, []byte(digest(envelope)), 0600); e != nil {
		return e
	}
	if e = safefile.SyncDir(stage); e != nil {
		return e
	}
	if e = safefile.SyncDir(dir); e != nil {
		return e
	}
	// The claim is durably single-target before admission is opened. Missing or
	// partial data always keeps the target sealed and is recoverable forward.
	return finishRestoredTarget(path, c.Target)
}
func finishRestoredTarget(path, target string) error {
	cfg, e := config.LoadFrom(path)
	if e != nil {
		return e
	}
	if cfg.InstanceID != target {
		return errors.New("restored installation identity mismatch")
	}
	for _, u := range cfg.Users {
		if u.SQLiteMeter == nil {
			continue
		}
		s, e := quota.Open(context.Background(), u.SQLiteMeter.Directory, cfg.ServerID, u.SQLiteMeter.Store)
		if e != nil {
			return e
		}
		_, e = s.Metadata(context.Background(), quota.Key{Account: u.AccountID, Period: u.PeriodID})
		s.Close()
		if e != nil {
			return e
		}
	}
	if cfg.SubscriptionV2 {
		for _, name := range []string{path + ".status-v2.anchor", filepath.Join(path+".status-v2", "records.json")} {
			if _, e = os.Stat(name); e != nil {
				return e
			}
		}
	}
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(path), "recovery-manifest.json"))
	if e != nil {
		return e
	}
	var m CoherentManifest
	if json.Unmarshal(raw, &m) != nil {
		return errors.New("recovery authority manifest lost")
	}
	if _, had := m.Files["status-revisions/records.json"]; had {
		for _, name := range []string{path + ".status-v2.anchor", filepath.Join(path+".status-v2", "records.json")} {
			if _, e = os.Stat(name); e != nil {
				return errors.New("recovered revision authority missing")
			}
		}
	}
	for name, hash := range m.Files {
		if strings.HasPrefix(name, "receipts/") {
			p := filepath.Join(filepath.Dir(path), "subscriptions/.rotation-receipts", filepath.Base(name))
			b, e := os.ReadFile(p)
			if e != nil || digest(b) != hash {
				return errors.New("recovery receipt authority missing")
			}
		}
	}
	if cfg.V2Authority != nil {
		if e = ValidateV2Authority(cfg); e != nil {
			return e
		}
	}
	if !cfg.RecoverySealed {
		return nil
	}
	cfg.RecoverySealed = false
	return cfg.Save()
}

func copyRecoveryDB(snapshot, name, to, expected string) error {
	raw, e := os.ReadFile(filepath.Join(snapshot, name, "ledger.sqlite"))
	if e != nil {
		return e
	}
	if digest(raw) != expected {
		return errors.New("database checkpoint mismatch")
	}
	if e = os.Mkdir(to, 0700); e != nil && !os.IsExist(e) {
		return e
	}
	target := filepath.Join(to, "ledger.sqlite")
	if prior, e := os.ReadFile(target); e == nil {
		if digest(prior) != expected {
			return errors.New("target database differs; preserve it for forward recovery")
		}
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = safefile.Write(target, raw, 0600); e != nil {
		return fmt.Errorf("recovery database: %w", e)
	}
	return safefile.SyncDir(to)
}
