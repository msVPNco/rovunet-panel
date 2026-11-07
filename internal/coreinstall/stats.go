package coreinstall

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/usage"
)

// REALITY and XHTTP cores expose Xray's statistics API on a loopback port.
// Xray keeps its counters in memory and clears them when the core restarts,
// so each reading is taken with -reset (a delta) and added to a running total
// saved in the usage directory.

const apiPortBase = 10085

// pickAPIPort chooses a free loopback port for a core's statistics API, not
// used by another core.
func pickAPIPort(self string) int {
	used := map[int]bool{}
	if cores, err := List(); err == nil {
		for _, c := range cores {
			if c.Name == self {
				continue
			}
			var st coreplan.State
			if readJSON(filepath.Join(Root, c.Name, stateFile), &st) == nil && st.APIPort > 0 {
				used[st.APIPort] = true
			}
		}
	}
	for p := apiPortBase; p < apiPortBase+500; p++ {
		if used[p] {
			continue
		}
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return p
	}
	return apiPortBase
}

// runStats runs `xray api statsquery`; variable for tests.
var runStats = func(port int, reset bool) ([]byte, error) {
	args := []string{"api", "statsquery", "-s", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), "-t", "5", "-pattern", "user>>>"}
	if reset {
		args = append(args, "-reset=true")
	}
	return exec.Command(XrayPath, args...).Output()
}

// parseStats sums each user's up and down bytes from statsquery output:
// {"stat":[{"name":"user>>>alice>>>traffic>>>uplink","value":"123"}]}.
func parseStats(out []byte) (map[string]int64, error) {
	var doc struct {
		Stat []struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		} `json:"stat"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("unreadable statistics: %w", err)
	}
	totals := map[string]int64{}
	for _, s := range doc.Stat {
		parts := strings.Split(s.Name, ">>>")
		if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
			continue
		}
		v, err := strconv.ParseInt(strings.Trim(string(s.Value), `"`), 10, 64)
		if err != nil || v < 0 {
			continue
		}
		totals[parts[1]] += v
	}
	return totals, nil
}

// corePort is the statistics port of a user core, or 0.
func corePort(name string) int {
	var st coreplan.State
	if readJSON(filepath.Join(Root, name, stateFile), &st) != nil {
		return 0
	}
	return st.APIPort
}

func lockUsage() (func(), error) {
	if err := os.MkdirAll(usage.Dir, 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(usage.Dir, "xray.lock"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// collectCore moves one core's counters into the running totals.
func collectCore(name string) error {
	port := corePort(name)
	if port == 0 {
		return nil
	}
	unlock, err := lockUsage()
	if err != nil {
		return err
	}
	defer unlock()
	out, err := runStats(port, true)
	if err != nil {
		return fmt.Errorf("core %s: statistics: %w", name, err)
	}
	delta, err := parseStats(out)
	if err != nil {
		return fmt.Errorf("core %s: %w", name, err)
	}
	if len(delta) == 0 {
		return nil
	}
	c := usage.NewCounter("xray")
	for user, n := range delta {
		c.Add(user, n)
	}
	return c.Flush()
}

// CollectUsage reads every metered core's counters into the running totals.
func CollectUsage() error {
	cores, err := List()
	if err != nil {
		return err
	}
	var first error
	for _, c := range cores {
		if err := collectCore(c.Name); err != nil && first == nil {
			first = err
		}
	}
	return first
}

var (
	workingMu    sync.Mutex
	workingCache = map[string]bool{}
)

// StatsWorking reports whether a core's statistics answer, which is what
// lets a traffic-limited user be given that core's config. It is checked
// once per process.
func StatsWorking(name string) bool {
	workingMu.Lock()
	defer workingMu.Unlock()
	if v, ok := workingCache[name]; ok {
		return v
	}
	port := corePort(name)
	ok := false
	if port > 0 {
		done := make(chan bool, 1)
		go func() {
			out, err := runStats(port, false)
			if err != nil {
				done <- false
				return
			}
			_, perr := parseStats(out)
			done <- perr == nil
		}()
		select {
		case ok = <-done:
		case <-time.After(8 * time.Second):
		}
	}
	workingCache[name] = ok
	return ok
}
