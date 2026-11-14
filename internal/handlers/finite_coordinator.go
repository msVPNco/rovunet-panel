package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

type finiteUpgradePlan struct {
	Version                                  int
	Server, Operation, Backup, Native, Phase string
	Before                                   []byte
	Units, Active, Cores, Users              []string
}

var finiteMigrationBarrier func(string) error

func finiteBarrier(phase string) error {
	if finiteMigrationBarrier != nil {
		return finiteMigrationBarrier(phase)
	}
	return nil
}

var finiteControl = func(action, unit string) error {
	if _, e := exec.Command("systemctl", action, unit).CombinedOutput(); e != nil {
		return fmt.Errorf("finite upgrade %s %s failed: %w", action, unit, e)
	}
	return nil
}

func MigrateFiniteAccounting(cfg *config.Config, out actions.OutputWriter) error {
	pending := filepath.Join(cfg.Path()+".finite-upgrade", "pending.json")
	needed := false
	for _, u := range cfg.Users {
		if u.QuotaGB > 0 && u.SQLiteMeter == nil && !u.Expired(time.Now()) {
			needed = true
		}
	}
	// New bridge-capable cores added by an update also need attachment, even
	// when all accounts were bound by a previous successful migration.
	hasFinite := false
	for _, u := range cfg.Users {
		if u.QuotaGB > 0 && !u.Expired(time.Now()) {
			hasFinite = true
		}
	}
	if hasFinite && !needed {
		cores, e := coreinstall.List()
		if e != nil {
			return e
		}
		for _, c := range cores {
			if coreplan.BridgeProtocol(c.Protocol) {
				b, e := os.ReadFile(filepath.Join(coreinstall.Root, c.Name, "options.json"))
				if e != nil {
					return e
				}
				var o coreplan.Options
				if e = json.Unmarshal(b, &o); e != nil {
					return e
				}
				if o.QuotaBridge == nil {
					needed = true
				}
			}
		}
	}
	if _, e := os.Stat(pending); !needed && os.IsNotExist(e) {
		return nil
	}
	root := filepath.Dir(pending)
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	lock, e := os.OpenFile(filepath.Join(root, "write.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if e = cfg.RefreshIfChanged(); e != nil {
		return e
	}
	var plan finiteUpgradePlan
	persist := func() error {
		b, e := json.MarshalIndent(plan, "", "  ")
		if e != nil {
			return e
		}
		return safefile.Write(pending, b, 0600)
	}
	if b, re := os.ReadFile(pending); re == nil {
		if json.Unmarshal(b, &plan) != nil || plan.Version != 1 || plan.Server != cfg.ServerID || plan.Operation == "" {
			return errors.New("invalid finite upgrade receipt")
		}
		// User identities and limits cannot change underneath a partially completed migration.
		var before config.Config
		if json.Unmarshal(plan.Before, &before) != nil {
			return errors.New("invalid finite upgrade source")
		}
		if len(before.Users) != len(cfg.Users) {
			return errors.New("account list changed during finite migration")
		}
		for _, old := range before.Users {
			now := cfg.GetUser(old.Username)
			if now == nil || old.AccountID != now.AccountID || old.PeriodID != now.PeriodID || old.SubToken != now.SubToken || old.Password != now.Password || old.QuotaGB != now.QuotaGB || old.UsageBase != now.UsageBase || old.ExpiresAt != now.ExpiresAt {
				return errors.New("account changed during finite migration; recover forward")
			}
		}
	} else if !os.IsNotExist(re) {
		return re
	} else {
		// Verify prerequisites before stopping anything.
		for _, t := range cfg.Tunnels {
			if t.Enabled && t.VLESS != nil {
				if plan.Native != "" || cfg.VLESSFront(&t) == nil {
					return errors.New("finite upgrade needs one loopback TLS-fronted VLESS listener")
				}
				plan.Native = t.Tag
			}
		}
		if plan.Native == "" {
			return errors.New("finite upgrade native listener missing")
		}
		for _, u := range cfg.Users {
			if u.QuotaGB > 0 && !u.Expired(time.Now()) {
				if !validPublicationID(u.AccountID, u.PeriodID, u.SubToken) {
					return errors.New("finite upgrade account identity missing")
				}
				plan.Users = append(plan.Users, u.Username)
			}
		}
		plan.Version = 1
		plan.Server = cfg.ServerID
		plan.Operation = config.NewID()
		plan.Phase = "planned"
		plan.Before, e = os.ReadFile(cfg.Path())
		if e != nil {
			return e
		}
		plan.Backup, e = setupBackup(cfg)
		if e != nil {
			return e
		}
		plan.Units = []string{"rovunet-user-expiry.timer", service.TunnelServiceName(plan.Native) + ".service"}
		cores, e := coreinstall.List()
		if e != nil {
			return e
		}
		for _, c := range cores {
			if coreplan.BridgeProtocol(c.Protocol) {
				plan.Cores = append(plan.Cores, c.Name)
				plan.Units = append(plan.Units, coreinstall.ServiceName(c.Name)+".service")
			}
		}
		for _, u := range plan.Units {
			if exec.Command("systemctl", "is-active", "--quiet", u).Run() == nil {
				plan.Active = append(plan.Active, u)
			}
		}
		if e = persist(); e != nil {
			return e
		}
	}
	if plan.Phase != "ready" && plan.Phase != "complete" {
		out.Info("Finite accounting migration: stopping legacy paths for a durable opening; original URLs and byte limits are retained")
		// Historical gap errors do not justify discarding the successfully sampled lower bound.
		_ = coreinstall.CollectUsage()
		for _, unit := range plan.Units {
			if e = finiteControl("stop", unit); e != nil {
				return e
			}
		}
		if e = finiteBarrier("stopped"); e != nil {
			return e
		}
		if e = BindFiniteAccounting(cfg); e != nil {
			return e
		}
		if e = finiteBarrier("bound"); e != nil {
			return e
		}
		if e = AttachFiniteBridges(cfg); e != nil {
			return e
		}
		for i := range cfg.Tunnels {
			t := &cfg.Tunnels[i]
			if t.Tag == plan.Native {
				if e = transport.RegenerateService(t, cfg); e != nil {
					return e
				}
			}
		}
		for _, name := range plan.Cores {
			// Validate before making the receipt boot-admissible. Disabled cores remain disabled.
			var state coreplan.State
			b, e := os.ReadFile(filepath.Join(coreinstall.Root, name, "state.json"))
			if e != nil {
				return e
			}
			if e = json.Unmarshal(b, &state); e != nil {
				return e
			}
			if !state.Disabled {
				if e = coreinstall.Prepare(name, cfg.Users); e != nil {
					return e
				}
			}
			if e = service.Create(coreinstall.Unit(name, filepath.Join(coreinstall.Root, name))); e != nil {
				return e
			}
		}
		if e = finiteBarrier("prepared"); e != nil {
			return e
		}
		plan.Phase = "ready"
		if e = persist(); e != nil {
			return e
		}
		if e = finiteBarrier("ready"); e != nil {
			return e
		}
	}
	// Restart only the units that were active before this operation; timer last.
	for _, unit := range plan.Active {
		if unit == "rovunet-user-expiry.timer" {
			continue
		}
		if e = finiteControl("start", unit); e != nil {
			return e
		}
	}
	if e = finiteBarrier("started"); e != nil {
		return e
	}
	if e = WriteSubscriptions(cfg, out); e != nil {
		return e
	}
	for _, unit := range plan.Active {
		if unit == "rovunet-user-expiry.timer" {
			if e = finiteControl("start", unit); e != nil {
				return e
			}
		}
	}
	plan.Phase = "complete"
	if e = persist(); e != nil {
		return e
	}
	if e = finiteBarrier("complete"); e != nil {
		return e
	}
	if e = os.Rename(pending, filepath.Join(root, plan.Operation+".complete.json")); e != nil {
		return e
	}
	if e = safefile.SyncDir(root); e != nil {
		return e
	}
	out.Success("Finite accounting migrated; previous account identities, subscription URLs and recorded usage preserved")
	return nil
}
