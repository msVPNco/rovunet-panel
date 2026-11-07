// Package usage counts the traffic each user moves through the panel's own
// VLESS and SOCKS5 servers, so a traffic limit can be enforced. Each server
// keeps running totals in its own file (bytes, both directions, forever) and
// the limit check sums them. Other protocols (SSH, NaïveProxy, DNS tunnels,
// Xray cores) are not measured here.
package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Dir holds one totals file per server. Variable for tests.
var Dir = "/var/lib/rovunet/usage"

// FlushEvery is how often a running server writes its totals.
const FlushEvery = 15 * time.Second

// Counter is the running total of one server.
type Counter struct {
	path  string
	mu    sync.Mutex
	m     map[string]int64
	dirty bool
}

// NewCounter starts from the totals the named server saved earlier, so a
// restart does not lose them.
func NewCounter(name string) *Counter {
	c := &Counter{path: filepath.Join(Dir, name+".json"), m: map[string]int64{}}
	if data, err := os.ReadFile(c.path); err == nil {
		_ = json.Unmarshal(data, &c.m)
		if c.m == nil {
			c.m = map[string]int64{}
		}
	}
	return c
}

// Add counts n bytes for user.
func (c *Counter) Add(user string, n int64) {
	if user == "" || n <= 0 {
		return
	}
	c.mu.Lock()
	c.m[user] += n
	c.dirty = true
	c.mu.Unlock()
}

// Flush writes the totals if they changed.
func (c *Counter) Flush() error {
	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()
		return nil
	}
	data, err := json.Marshal(c.m)
	c.dirty = false
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(Dir, 0755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// Run flushes every FlushEvery and once more when ctx ends.
func (c *Counter) Run(ctx context.Context) {
	t := time.NewTicker(FlushEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_ = c.Flush()
		case <-ctx.Done():
			_ = c.Flush()
			return
		}
	}
}

// Totals sums every server's totals per user.
func Totals() map[string]int64 {
	total := map[string]int64{}
	entries, _ := os.ReadDir(Dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(Dir, e.Name()))
		if err != nil {
			continue
		}
		var m map[string]int64
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		for u, n := range m {
			total[u] += n
		}
	}
	return total
}

// restoredName is the totals file a restore writes. No server owns it.
const restoredName = "restored"

// totalsExcept sums every totals file but the named one.
func totalsExcept(skip string) map[string]int64 {
	total := map[string]int64{}
	entries, _ := os.ReadDir(Dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == skip+".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(Dir, e.Name()))
		if err != nil {
			continue
		}
		var m map[string]int64
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		for u, n := range m {
			total[u] += n
		}
	}
	return total
}

// Restore makes the totals at least the ones from a backup. It adds only what
// this server lacks, in a file of its own, so restoring twice, or onto a
// server that already counted some of the traffic, never counts it twice.
func Restore(saved map[string]int64) error {
	have := totalsExcept(restoredName)
	extra := map[string]int64{}
	for u, n := range saved {
		if n > have[u] {
			extra[u] = n - have[u]
		}
	}
	if err := os.MkdirAll(Dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(extra)
	if err != nil {
		return err
	}
	path := filepath.Join(Dir, restoredName+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
