package config

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"os"
	"syscall"
)

// CommitQuiescedFile is the coordinator's CAS+fence primitive. work may touch
// the database and Config fields directly; it must not call config methods.
func (c *Config) CommitQuiescedFile(work func(*Config) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		return ErrConcurrentChange
	}
	f, e := os.OpenFile(c.path+".write.lock", os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	old, e := os.ReadFile(c.path)
	if e != nil {
		return e
	}
	if sha256.Sum256(old) != c.diskSHA {
		return ErrConcurrentChange
	}
	// Only affected users are fenced by the coordinator's same-user operation.
	return c.quiesceNames([]string{c.carryUser()}, func() error {
		if e = work(c); e != nil {
			return e
		}
		data, e := json.MarshalIndent(c, "", "  ")
		if e != nil {
			return e
		}
		data = append(data, '\n')
		if e = safefile.Write(c.path, data, 0600); e != nil {
			return e
		}
		c.diskSHA = sha256.Sum256(data)
		return nil
	})
}
func (c *Config) carryUser() string {
	b, e := os.ReadFile(c.path + ".carry.pending")
	if e != nil {
		return ""
	}
	var marker struct{ User string }
	if json.Unmarshal(b, &marker) != nil {
		return ""
	}
	return marker.User
}
func AccountOnHold(path, user string) bool {
	b, e := os.ReadFile(path + ".carry.pending")
	if os.IsNotExist(e) {
		return false
	}
	if e != nil {
		return true
	}
	var marker struct{ User string }
	if json.Unmarshal(b, &marker) != nil || marker.User == "" {
		return true
	}
	return marker.User == user
}

var ErrCoordinatedRecovery = errors.New("coordinated operation pending; recover forward before changing configuration")

// HasCarryHold reads the immutable loaded path and is safe inside a
// WithCurrentFile callback; it deliberately does not re-enter the config mutex.
func (c *Config) HasCarryHold(user string) bool { return AccountOnHold(c.path, user) }
