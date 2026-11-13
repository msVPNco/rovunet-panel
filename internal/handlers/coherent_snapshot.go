package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

type SnapshotDB struct {
	Server, Store, Directory, Snapshot string
	Accounts                           []quota.Key
}
type CoherentManifest struct {
	NativeSecrets                 map[string]string
	Version                       int
	Operation, Server, ConfigHash string
	Databases                     []SnapshotDB
	Files                         map[string]string
}

func requiresCoherent(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.V2Authority != nil {
		return true
	}
	for _, u := range cfg.Users {
		if u.SQLiteMeter != nil {
			return true
		}
	}
	for _, u := range cfg.Archived {
		if u.SQLiteMeter != nil {
			return true
		}
	}
	entries, e := os.ReadDir(receiptDir())
	return (e != nil && !os.IsNotExist(e)) || len(entries) > 0
}
func privateSnapshotFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.New("nonregular snapshot member")
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		if rel == "manifest.json" {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		files[filepath.ToSlash(rel)] = digest(b)
		return nil
	})
	return files, e
}

// Snapshot is a private lab export; its manifest hash must be retained in
// authenticated operator storage. It contains credentials and is not a release.
func CreateCoherentSnapshot(ctx context.Context, cfg *config.Config, dest, operation string) (string, error) {
	if !tokenOK(operation) {
		return "", errors.New("invalid operation ID")
	}
	configPath := cfg.Path()
	var result string
	e := withPublicationLock(func() error {
		if e := cfg.RefreshIfChanged(); e != nil {
			return e
		}
		if _, e := resumeRotation(cfg); e != nil && !errors.Is(e, errSuperseded) {
			return e
		}
		return cfg.WithCurrentFile(func() error {
			return cfg.Quiesce(func() error {
				if e := os.Mkdir(dest, 0700); e != nil {
					return e
				}
				raw, e := os.ReadFile(configPath)
				if e != nil {
					return e
				}
				if e = safefile.Write(filepath.Join(dest, "config.json"), raw, 0600); e != nil {
					return e
				}
				m := CoherentManifest{Version: 1, Operation: operation, Server: cfg.ServerID, ConfigHash: digest(raw)}
				m.NativeSecrets = map[string]string{}
				for _, t := range cfg.Tunnels {
					if t.VLESS == nil || t.VLESS.SecretFile == "" {
						continue
					}
					info, e := os.Lstat(t.VLESS.SecretFile)
					if e != nil {
						return e
					}
					if !info.Mode().IsRegular() || info.Size() > 1<<20 {
						return errors.New("invalid native recovery secret")
					}
					secret, e := os.ReadFile(t.VLESS.SecretFile)
					if e != nil {
						return e
					}
					name := "native-" + digest([]byte(t.Tag)) + ".secret"
					if e = os.MkdirAll(filepath.Join(dest, "runtime"), 0700); e != nil {
						return e
					}
					if e = safefile.Write(filepath.Join(dest, "runtime", name), secret, 0600); e != nil {
						return e
					}
					m.NativeSecrets[t.Tag] = name
				}
				dbs := map[string]*SnapshotDB{}
				all := append([]config.UserConfig(nil), cfg.Users...)
				for _, a := range cfg.Archived {
					all = append(all, a.UserConfig)
				}
				for _, u := range all {
					if u.SQLiteMeter == nil {
						continue
					}
					b := u.SQLiteMeter
					key := b.Directory + "/" + b.Store
					db := dbs[key]
					if db == nil {
						db = &SnapshotDB{Server: cfg.ServerID, Store: b.Store, Directory: b.Directory, Snapshot: fmt.Sprintf("db-%d", len(dbs))}
						dbs[key] = db
					}
					db.Accounts = append(db.Accounts, quota.Key{Account: u.AccountID, Period: u.PeriodID})
				}
				for _, db := range dbs {
					s, e := quota.Open(ctx, db.Directory, db.Server, db.Store)
					if e != nil {
						return e
					}
					e = s.Backup(ctx, filepath.Join(dest, db.Snapshot))
					s.Close()
					if e != nil {
						return e
					}
					m.Databases = append(m.Databases, *db)
				}
				sort.Slice(m.Databases, func(i, j int) bool { return m.Databases[i].Snapshot < m.Databases[j].Snapshot })
				if e = copyGeneration(publicDir(), filepath.Join(dest, "public")); e != nil {
					return e
				}
				if _, e = os.Stat(receiptDir()); e == nil {
					if e = copyGeneration(receiptDir(), filepath.Join(dest, "receipts")); e != nil {
						return e
					}
					if e = os.Chmod(filepath.Join(dest, "receipts"), 0700); e != nil {
						return e
					}
				}
				if _, e = os.Stat(configPath + ".carry"); e == nil {
					if e = copyGeneration(configPath+".carry", filepath.Join(dest, "carry-operations")); e != nil {
						return e
					}
				}
				if _, e = os.Stat(configPath + ".status-v2.adoption"); e == nil {
					if e = copyPrivateTree(configPath+".status-v2.adoption", filepath.Join(dest, "revision-adoption")); e != nil {
						return e
					}
				}
				if _, e = os.Stat(configPath + ".status-v2"); e == nil {
					lock, e := os.OpenFile(filepath.Join(configPath+".status-v2", "write.lock"), os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
					if e != nil {
						return e
					}
					if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
						lock.Close()
						return e
					}
					e = copyGeneration(configPath+".status-v2", filepath.Join(dest, "status-revisions"))
					if e == nil {
						anchor, err := os.ReadFile(configPath + ".status-v2.anchor")
						if err != nil {
							e = err
						} else {
							e = safefile.Write(filepath.Join(dest, "status-revisions.anchor"), anchor, 0600)
						}
					}
					lock.Close()
					if e != nil {
						return e
					}
				}
				m.Files, e = privateSnapshotFiles(dest)
				if e != nil {
					return e
				}
				raw, e = json.MarshalIndent(m, "", "  ")
				if e != nil {
					return e
				}
				if e = safefile.Write(filepath.Join(dest, "manifest.json"), raw, 0600); e != nil {
					return e
				}
				if e = safefile.SyncDir(dest); e != nil {
					return e
				}
				if e = safefile.SyncDir(filepath.Dir(dest)); e != nil {
					return e
				}
				result = digest(raw)
				return nil
			})
		})
	})
	return result, e
}

// Reconcile is forward-only into an existing same-identity installation.
// It never rolls back config/tokens/receipts or replaces a database with an
// older file. Missing current authority requires offline fresh-period cutover.
func ReconcileCoherentSnapshot(ctx context.Context, cfg *config.Config, dir, expected string) error {
	configPath := cfg.Path()
	raw, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return e
	}
	if digest(raw) != expected {
		return quota.ErrConflict
	}
	var m CoherentManifest
	if e = json.Unmarshal(raw, &m); e != nil || m.Version != 1 || !tokenOK(m.Operation) {
		return quota.ErrConflict
	}
	actual, e := privateSnapshotFiles(dir)
	if e != nil {
		return e
	}
	a, _ := json.Marshal(actual)
	b, _ := json.Marshal(m.Files)
	if string(a) != string(b) {
		return errors.New("snapshot file hash mismatch")
	}
	return withPublicationLock(func() error {
		if e = cfg.RefreshIfChanged(); e != nil {
			return e
		}
		return cfg.WithCurrentFile(func() error {
			return cfg.Quiesce(func() error {
				live, e := os.ReadFile(configPath)
				if e != nil {
					return e
				}
				if cfg.ServerID != m.Server || digest(live) != m.ConfigHash {
					return errors.New("snapshot predates a configuration/capability change; forward migration required")
				}
				for name, hash := range m.Files {
					if !strings.HasPrefix(name, "receipts/") {
						continue
					}
					data, e := os.ReadFile(filepath.Join(receiptDir(), filepath.Base(name)))
					if e != nil || digest(data) != hash {
						return errors.New("current durable receipt authority unavailable; refusing rollback")
					}
				}
				for _, db := range m.Databases {
					if db.Server != cfg.ServerID || filepath.Base(db.Snapshot) != db.Snapshot || !strings.HasPrefix(db.Snapshot, "db-") {
						return quota.ErrConflict
					}
					s, e := quota.Open(ctx, db.Directory, db.Server, db.Store)
					if e != nil {
						return e
					}
					for _, k := range db.Accounts {
						e = s.MergeBackup(ctx, k, filepath.Join(dir, db.Snapshot), m.Files[db.Snapshot+"/ledger.sqlite"])
						if e != nil {
							s.Close()
							return e
						}
					}
					s.Close()
				}
				return nil
			})
		})
	})
}
