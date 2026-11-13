package config

import (
	"sort"
	"time"
)

// Quiesce is shared by coherent snapshot/recovery. Caller already owns the
// configuration/publication CAS. Existing I/O must drain before work begins.
func (c *Config) Quiesce(work func() error) error {
	names := map[string]bool{}
	for _, u := range c.Users {
		if u.QuotaGB > 0 {
			names[u.Username] = true
		}
	}
	ordered := make([]string, 0, len(names))
	for n := range names {
		ordered = append(ordered, n)
	}
	sort.Strings(ordered)
	return c.quiesceNames(ordered, work)
}
func (c *Config) quiesceNames(ordered []string, work func() error) error {
	for _, name := range ordered {
		if name == "" {
			return ErrCoordinatedRecovery
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for _, kind := range []string{"gate", "live"} {
		for _, name := range ordered {
			f, e := accessLock(c.path, name, kind)
			if e != nil {
				return e
			}
			defer f.Close()
			if e = exclusive(f, deadline); e != nil {
				return e
			}
		}
	}
	return work()
}
