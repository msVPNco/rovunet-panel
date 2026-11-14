package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

// BindFiniteAccounting is the cold part of migration. The coordinator must
// stop all legacy paths before taking the opening snapshot; a later replay
// cannot import a different opening or create a second period. Unknown past
// bytes remain unknown; only the recorded lower bound is carried forward.
func BindFiniteAccounting(cfg *config.Config) error {
	path := cfg.Path()
	if cfg.RecoverySealed || cfg.HasCarryHold("") {
		return config.ErrCoordinatedRecovery
	}
	if err := cfg.ValidateSubscriptionTokens(); err != nil {
		return err
	}
	return withPublicationLock(func() error {
		if e := cfg.RefreshIfChanged(); e != nil {
			return e
		}
		e := cfg.WithCurrentFile(func() error {
			return cfg.Quiesce(func() error {
				var native *config.TunnelConfig
				for i := range cfg.Tunnels {
					t := &cfg.Tunnels[i]
					if t.Enabled && t.VLESS != nil {
						if native != nil {
							return errors.New("finite migration requires one native quota listener")
						}
						native = t
					}
				}
				fronted := false
				if native != nil {
					for _, t := range cfg.Tunnels {
						if t.Tag == native.VLESS.Front && t.Enabled && t.Transport == config.TransportNaive && t.Naive != nil {
							fronted = true
						}
					}
				}
				if !fronted {
					return errors.New("finite migration needs the existing loopback TLS-fronted native listener")
				}
				dir := finiteLedgerDirectory(cfg, path)
				root := filepath.Dir(dir)
				storeID := digest([]byte(cfg.ServerID + "/finite-pool-v1"))[:32]
				if e := os.MkdirAll(root, 0700); e != nil {
					return e
				}
				raw, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				var top map[string]json.RawMessage
				if e = json.Unmarshal(raw, &top); e != nil {
					return e
				}
				var rows []map[string]json.RawMessage
				if e = json.Unmarshal(top["users"], &rows); e != nil || len(rows) != len(cfg.Users) {
					return errors.New("account list changed")
				}
				bootstrap := false
				var store *quota.Store
				defer func() {
					if store != nil {
						store.Close()
					}
				}()
				changed := false
				for i, u := range cfg.Users {
					if u.QuotaGB <= 0 || u.Expired(time.Now()) {
						continue
					}
					if !validPublicationID(cfg.ServerID, u.AccountID, u.PeriodID, u.SubToken) {
						return errors.New("persist account identities before finite migration")
					}
					if u.SQLiteMeter != nil {
						if u.SQLiteMeter.Directory != dir || u.SQLiteMeter.Store != storeID {
							return errors.New("existing finite ledger needs explicit reconciliation; it will not be replaced")
						}
						if _, _, e := boundAmount(cfg.ServerID, u); e != nil {
							return e
						}
						continue
					}
					snap, e := usage.RecordedSnapshot(u.Username)
					if errors.Is(e, usage.ErrNoAccountEvidence) {
						fresh, fe := recordedNewPeriod(path, u)
						if fe != nil {
							return fe
						}
						if !fresh {
							return errors.New("missing legacy usage evidence; migration cannot assume zero")
						}
						e = nil
					}
					if e != nil {
						return e
					}
					if snap.Total < u.UsageBase {
						return errors.New("recorded total below original usage baseline")
					}
					opening := snap.Total - u.UsageBase
					// Published floors may be newer than a restored counter. Never regress them.
					for _, suffix := range []string{".status.json", ".windows.status.json"} {
						b, re := os.ReadFile(filepath.Join(publicDir(), u.SubToken+suffix))
						if re != nil && !os.IsNotExist(re) {
							return re
						}
						if re == nil {
							var st accountStatus
							if json.Unmarshal(b, &st) != nil {
								return errors.New("invalid previous account status")
							}
							if st.AccountID == u.AccountID && st.PeriodID == u.PeriodID && st.UsedBytes != nil && *st.UsedBytes > opening {
								opening = *st.UsedBytes
							}
						}
					}
					cp := quota.Checkpoint{Server: cfg.ServerID, Store: storeID, Account: u.AccountID, Period: u.PeriodID, ID: "finite-opening-" + u.PeriodID, OpeningOrigin: "legacy-" + u.PeriodID, Proof: "cold-recorded-lower-bound:" + snap.Fingerprint, Opening: opening}
					receipt := filepath.Join(root, u.AccountID+"-"+u.PeriodID+".opening.json")
					if b, re := os.ReadFile(receipt); re == nil {
						var old quota.Checkpoint
						if json.Unmarshal(b, &old) != nil || old.Server != cp.Server || old.Store != cp.Store || old.Account != cp.Account || old.Period != cp.Period || old.Opening != cp.Opening {
							return errors.New("opening changed after partial migration; recover forward")
						}
						cp = old
					} else if !os.IsNotExist(re) {
						return re
					} else {
						b, _ := json.Marshal(cp)
						if e = safefile.Write(receipt, b, 0600); e != nil {
							return e
						}
					}
					if store == nil {
						store, e = quota.Open(context.Background(), dir, cfg.ServerID, storeID)
						if e != nil {
							if _, se := os.Stat(dir); os.IsNotExist(se) {
								store, e = quota.Create(context.Background(), dir, cfg.ServerID, storeID)
							} else {
								store, e = quota.ResumeBootstrap(context.Background(), dir, cfg.ServerID, storeID)
							}
							if e != nil {
								return e
							}
							bootstrap = true // Bootstrap is sealed after importing all accounts below.
						}
					}
					k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
					if e = store.NewPeriod(context.Background(), k, u.QuotaBytes()); e != nil {
						return e
					}
					if e = store.ImportOpening(context.Background(), cp); e != nil {
						return e
					}
					rows[i]["sqlite_meter"], e = json.Marshal(config.SQLiteBinding{Directory: dir, Store: storeID})
					if e != nil {
						return e
					}
					changed = true
				}
				if !changed {
					return nil
				}
				if bootstrap {
					if e = store.FinishBootstrap(dir); e != nil {
						return e
					}
				}
				top["users"], e = json.Marshal(rows)
				if e != nil {
					return e
				}
				raw, e = json.MarshalIndent(top, "", "  ")
				if e != nil {
					return e
				}
				return safefile.Write(path, append(raw, '\n'), 0600)
			})
		})
		if e != nil {
			return e
		}
		return cfg.RefreshIfChanged()
	})
}

// The installed configuration directory is legitimately owned by the service
// account. Durable root quota authority must not sit below that writable
// ancestor. Existing bound stores retain their identity and secure-path checks.
func finiteLedgerDirectory(cfg *config.Config, path string) string {
	for _, u := range cfg.Users {
		if u.QuotaGB > 0 && u.SQLiteMeter != nil {
			return u.SQLiteMeter.Directory
		}
	}
	if path == config.DefaultConfigFile {
		return filepath.Join("/var/lib/rovunet-quota", "finite-"+digest([]byte(cfg.ServerID))[:32], "db")
	}
	return filepath.Join(path+".finite-accounting", "db")
}

// AttachFiniteBridges changes only internal routing. Existing listener keys,
// addresses, paths and per-user credentials remain unchanged.
func AttachFiniteBridges(cfg *config.Config) error {
	var binding *config.SQLiteBinding
	for _, u := range cfg.Users {
		if u.QuotaGB > 0 && !u.Expired(time.Now()) {
			if u.SQLiteMeter == nil {
				return errors.New("unbound finite account on native SQL listener")
			}
			if binding != nil && *binding != *u.SQLiteMeter {
				return errors.New("finite accounts require one reviewed store")
			}
			binding = u.SQLiteMeter
		}
	}
	if binding == nil {
		return nil
	}
	var native *config.TunnelConfig
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if t.Enabled && t.VLESS != nil {
			if native != nil {
				return errors.New("multiple native quota listeners")
			}
			native = t
		}
	}
	if native == nil || cfg.VLESSFront(native) == nil {
		return errors.New("native quota listener unavailable")
	}
	bridge := &coreplan.NativeQuotaBridge{Port: native.VLESS.Port, Path: native.VLESS.Path, SecretFile: native.VLESS.SecretFile, Directory: binding.Directory, Store: binding.Store}
	cores, e := coreinstall.List()
	if e != nil {
		return e
	}
	for _, c := range cores {
		if !coreplan.BridgeProtocol(c.Protocol) {
			continue
		}
		p := filepath.Join(coreinstall.Root, c.Name, "options.json")
		raw, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var o coreplan.Options
		if e = json.Unmarshal(raw, &o); e != nil {
			return e
		}
		o.QuotaBridge = bridge
		o.DedicatedXray = true
		next, e := json.MarshalIndent(o, "", "  ")
		if e != nil {
			return e
		}
		if e = safefile.Write(p, next, 0600); e != nil {
			return fmt.Errorf("attach bridge %s: %w", c.Name, e)
		}
	}
	return nil
}

// finiteBridgeOptions is also used while provisioning a new core after a
// completed migration. A later update must not silently omit its finite users.
func finiteBridgeOptions(cfg *config.Config) (*coreplan.NativeQuotaBridge, error) {
	var binding *config.SQLiteBinding
	for _, u := range cfg.Users {
		if u.QuotaGB <= 0 || u.Expired(time.Now()) {
			continue
		}
		if u.SQLiteMeter == nil {
			return nil, nil
		}
		if binding != nil && *binding != *u.SQLiteMeter {
			return nil, errors.New("finite bridge store conflict")
		}
		binding = u.SQLiteMeter
	}
	if binding == nil {
		return nil, nil
	}
	var native *config.TunnelConfig
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if t.Enabled && t.VLESS != nil {
			if native != nil {
				return nil, errors.New("multiple native quota listeners")
			}
			native = t
		}
	}
	if native == nil || cfg.VLESSFront(native) == nil {
		return nil, errors.New("native quota listener unavailable")
	}
	return &coreplan.NativeQuotaBridge{Port: native.VLESS.Port, Path: native.VLESS.Path, SecretFile: native.VLESS.SecretFile, Directory: binding.Directory, Store: binding.Store}, nil
}
