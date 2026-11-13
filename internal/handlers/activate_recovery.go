package handlers

import (
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/recoveryauth"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"os"
	"path/filepath"
)

// ActivateRestoredInstallation only populates an empty canonical installation.
// Replays verify the same physical target; no live database/config is replaced.
func ActivateRestoredInstallation(target string, envelope []byte, pinned string) error {
	var claim RecoveryClaim
	if e := recoveryauth.Verify(envelope, pinned, &claim); e != nil {
		return e
	}
	if claim.Status != "claimed" {
		return errors.New("recovery claim required")
	}
	receipt, e := os.ReadFile(filepath.Join(target, "restore.receipt"))
	if e != nil || string(receipt) != digest(envelope) {
		return errors.New("target restore is not committed")
	}
	state := filepath.Join(target, "state")
	c, e := config.LoadFrom(filepath.Join(state, "config.json"))
	if e != nil {
		return e
	}
	if c.InstanceID != claim.Target || c.RecoverySealed {
		return errors.New("target is not ready")
	}
	if live, e := config.Load(); e == nil {
		if live.InstanceID != c.InstanceID {
			return errors.New("live installation exists; activation refused")
		}
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = os.MkdirAll(config.DefaultConfigDir, 0700); e != nil {
		return e
	}
	for _, pair := range [][2]string{{filepath.Join(state, "subscriptions"), transport.SubscriptionDir}, {filepath.Join(state, "config.json.status-v2"), config.DefaultConfigFile + ".status-v2"}, {filepath.Join(state, "config.json.carry"), config.DefaultConfigFile + ".carry"}, {filepath.Join(state, "config.json.status-v2.adoption"), config.DefaultConfigFile + ".status-v2.adoption"}} {
		if _, e = os.Stat(pair[0]); os.IsNotExist(e) {
			continue
		} else if e != nil {
			return e
		}
		if e = copyPrivateTree(pair[0], pair[1]); e != nil {
			return e
		}
	}
	if b, e := os.ReadFile(filepath.Join(state, "config.json.status-v2.anchor")); e == nil {
		if e = safefile.Write(config.DefaultConfigFile+".status-v2.anchor", b, 0600); e != nil {
			return e
		}
	}
	if e = os.Chmod(transport.SubscriptionDir, 0711); e != nil {
		return e
	}
	sub := filepath.Join(transport.SubscriptionDir, "sub")
	if entries, e := os.ReadDir(sub); e == nil {
		if e = os.Chmod(sub, 0711); e != nil {
			return e
		}
		for _, f := range entries {
			if !f.Type().IsRegular() {
				return errors.New("invalid recovered public entry")
			}
			if e = os.Chmod(filepath.Join(sub, f.Name()), 0644); e != nil {
				return e
			}
		}
	}
	c.SubscriptionV2 = false
	return c.SaveTo(config.DefaultConfigFile)
}
func copyPrivateTree(from, to string) error {
	if e := os.MkdirAll(to, 0700); e != nil {
		return e
	}
	entries, e := os.ReadDir(from)
	if e != nil {
		return e
	}
	for _, f := range entries {
		if f.Type()&os.ModeSymlink != 0 {
			return errors.New("unexpected recovery symlink")
		}
		src, dst := filepath.Join(from, f.Name()), filepath.Join(to, f.Name())
		if f.IsDir() {
			if e = copyPrivateTree(src, dst); e != nil {
				return e
			}
		} else {
			b, e := os.ReadFile(src)
			if e != nil {
				return e
			}
			if prior, e := os.ReadFile(dst); e == nil {
				if digest(prior) != digest(b) {
					return errors.New("unrelated object at recovery destination")
				}
				continue
			} else if !os.IsNotExist(e) {
				return e
			}
			if e = safefile.Write(dst, b, 0600); e != nil {
				return e
			}
		}
	}
	return safefile.SyncDir(to)
}
