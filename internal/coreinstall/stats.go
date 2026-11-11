package coreinstall

import (
	"encoding/json"
	"fmt"
	"math"
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
// Xray keeps counters in memory. Read absolute values and atomically persist
// both the last sample and durable totals. An engine epoch change preserves
// the totals and explicitly records that unsampled traffic is unknown.

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
var runStats = func(endpoint string, reset bool) ([]byte, error) {
	args := []string{"api", "statsquery", "-s", endpoint, "-t", "5", "-pattern", "user>>>"}
	if reset {
		args = append(args, "-reset=true")
	}
	return exec.Command(XrayPath, args...).Output()
}

var runEpoch = func(name string) (string, error) {
	out, err := exec.Command("systemctl", "show", ServiceName(name), "--property=InvocationID", "--value").Output()
	epoch := strings.TrimSpace(string(out))
	if err != nil || epoch == "" {
		return "", fmt.Errorf("cannot identify the running engine epoch")
	}
	return epoch, nil
}

// parseStats sums each user's up and down bytes from statsquery output:
// {"stat":[{"name":"user>>>alice>>>traffic>>>uplink","value":"123"}]}.
func parseStats(out []byte) (map[string]int64, error) {
	return parseStatsValues(out, false)
}

func parseStatsValues(out []byte, perDirection bool) (map[string]int64, error) {
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
		if err != nil || v < 0 || (parts[3] != "uplink" && parts[3] != "downlink") {
			return nil, fmt.Errorf("invalid per-user statistics")
		}
		key := parts[1]
		if perDirection {
			key += ">>>" + parts[3]
		}
		if v > math.MaxInt64-totals[key] {
			return nil, fmt.Errorf("per-user statistics overflow")
		}
		totals[key] += v
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

func apiEndpoint(st coreplan.State) string {
	if st.APISocket != "" {
		return "unix://" + st.APISocket
	}
	if st.APIPort > 0 {
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(st.APIPort))
	}
	return ""
}

func coreEndpoint(name string) string {
	var st coreplan.State
	if readJSON(filepath.Join(Root, name, stateFile), &st) != nil {
		return ""
	}
	return apiEndpoint(st)
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
	return collectCoreSample(name, false)
}

func collectCoreSample(name string, final bool) error {
	_, st, managed, err := loadUserCore(name)
	if err != nil {
		return err
	}
	if !managed || (!final && (st.Paused || st.Disabled)) {
		return nil
	}
	endpoint := coreEndpoint(name)
	if endpoint == "" {
		return nil
	}
	unlock, err := lockUsage()
	if err != nil {
		return err
	}
	defer unlock()
	epoch, err := runEpoch(name)
	if err != nil {
		return fmt.Errorf("core %s: %w", name, err)
	}
	out, err := runStats(endpoint, false)
	if err != nil {
		return fmt.Errorf("core %s: statistics: %w", name, err)
	}
	delta, err := parseStatsValues(out, true)
	if err != nil {
		return fmt.Errorf("core %s: %w", name, err)
	}
	return usage.RecordAbsolute("xray-"+name, epoch, delta)
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
	workingCache = map[string]workingResult{}
)

type workingResult struct {
	ok bool
	at time.Time
}

// StatsWorking reports whether a core's statistics answer, which is what
// lets a traffic-limited user be given an eligible core's config. The result
// is cached briefly so one feed refresh does not query the same core per user.
func StatsWorking(name string) bool {
	workingMu.Lock()
	defer workingMu.Unlock()
	if v, ok := workingCache[name]; ok && time.Since(v.at) < 5*time.Second {
		return v.ok
	}
	endpoint := coreEndpoint(name)
	ok := false
	if endpoint != "" {
		done := make(chan bool, 1)
		go func() {
			out, err := runStats(endpoint, false)
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
	workingCache[name] = workingResult{ok: ok, at: time.Now()}
	return ok
}
