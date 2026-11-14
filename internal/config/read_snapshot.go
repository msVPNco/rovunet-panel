package config

import (
	"crypto/sha256"
	"os"
	"syscall"
)

// VerifyCurrentSnapshot participates in the writer's lock without requesting
// filesystem write permission. The renderer has no authority to create locks.
func (c *Config) VerifyCurrentSnapshot() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.loaded {
		return ErrConcurrentChange
	}
	f, e := os.OpenFile(c.path+".write.lock", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_SH); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	b, e := os.ReadFile(c.path)
	if e != nil {
		return e
	}
	if sha256.Sum256(b) != c.diskSHA {
		return ErrConcurrentChange
	}
	return nil
}
