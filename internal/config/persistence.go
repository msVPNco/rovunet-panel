package config

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

var ErrConcurrentChange = errors.New("configuration changed in another command; reload and retry; no configuration was overwritten")

func (c *Config) saveLocked(path string, explicit bool) error {
	return c.saveObserved(path, explicit, nil)
}
func (c *Config) saveObserved(path string, explicit bool, observe func(string) error) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	path = absolute
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".write.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if c.loaded && filepath.Clean(c.path) == filepath.Clean(path) {
		if os.IsNotExist(err) || sha256.Sum256(old) != c.diskSHA {
			return ErrConcurrentChange
		}
	} else if err == nil && !explicit {
		return errors.New("existing configuration was not loaded; refusing to overwrite it")
	}
	if _, e := os.Lstat(path + ".carry.pending"); e == nil || !os.IsNotExist(e) {
		return ErrCoordinatedRecovery
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := fencedSave(path, old, c, func() error { return safefile.WriteObserved(path, data, 0600, observe) }); err != nil {
		return err
	}
	c.path = path
	c.loaded = true
	c.diskSHA = sha256.Sum256(data)
	return nil
}

// replaceLocked does not copy a mutex and clears fields removed from the file.
// Unmarshalling over an existing Config leaves absent omitempty fields intact.
func (c *Config) replaceLocked(f *Config) {
	c.Listen = f.Listen
	c.Tunnels = f.Tunnels
	c.Backends = f.Backends
	c.Users = f.Users
	c.Archived = f.Archived
	c.Route = f.Route
	c.Warp = f.Warp
	c.PublicAddress = f.PublicAddress
	c.ExtraAddressIDs = f.ExtraAddressIDs
	c.ExtraAddresses = f.ExtraAddresses
	c.ServerID = f.ServerID
	c.SubscriptionV2 = f.SubscriptionV2
	c.V2Authority = f.V2Authority
	c.InstanceID = f.InstanceID
	c.RecoverySealOperation = f.RecoverySealOperation
	c.RecoverySealed = f.RecoverySealed
	c.RouterLimits = f.RouterLimits
	c.path = f.path
	c.loaded = f.loaded
	c.diskSHA = f.diskSHA
}

// RefreshIfChanged prevents an older timer/config object from resurrecting a
// retired subscription URL after a serialized subscription writer runs.
func (c *Config) RefreshIfChanged() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		return nil
	}
	fresh, err := LoadFrom(c.path)
	if err != nil {
		return err
	}
	if fresh.diskSHA != c.diskSHA {
		c.replaceLocked(fresh)
	}
	return nil
}

func (c *Config) SaveObserved(observe func(string) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		return ErrConcurrentChange
	}
	return c.saveObserved(c.path, false, observe)
}
