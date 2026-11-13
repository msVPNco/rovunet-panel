package config

import (
	"crypto/sha256"
	"errors"
	"os"
	"syscall"
)

// WithCurrentFile serializes a brief filesystem publication with all existing
// config saves and rejects a changed source snapshot. The callback must not
// call config load/save or perform network I/O.
func (c *Config) WithCurrentFile(apply func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		return errors.New("publication requires a persisted config")
	}
	f, e := os.OpenFile(c.path+".write.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	content, e := os.ReadFile(c.path)
	if e != nil {
		return e
	}
	if sha256.Sum256(content) != c.diskSHA {
		return ErrConcurrentChange
	}
	return apply()
}

// WithSnapshotDigest identifies the persisted input from which render began.
func (c *Config) WithSnapshotDigest() ([32]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.loaded {
		return [32]byte{}, ErrConcurrentChange
	}
	return c.diskSHA, nil
}
