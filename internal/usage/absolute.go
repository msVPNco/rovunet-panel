package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsampledTraffic means the current reading and lifetime totals were
// committed, while a previous process/counter gap remains explicitly unknown.
// It is distinct from a failed read or durable write.
var ErrUnsampledTraffic = errors.New("engine epoch or counter changed: totals preserved, unsampled traffic is unknown")

// AbsoluteSource commits the last engine reading and the durable totals in
// one file. Reading an engine never resets its counters: a crash between the
// API call and the disk write can therefore be retried without losing bytes.
type AbsoluteSource struct {
	Epoch      string           `json:"epoch"`
	Last       map[string]int64 `json:"last"`
	Totals     map[string]int64 `json:"totals"`
	Incomplete bool             `json:"incomplete"`
}

func readAbsolute(path string) (AbsoluteSource, error) {
	var s AbsoluteSource
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	if s.Epoch == "" || s.Last == nil || s.Totals == nil {
		return s, fmt.Errorf("incomplete source ledger")
	}
	for _, values := range []map[string]int64{s.Last, s.Totals} {
		for _, n := range values {
			if n < 0 {
				return s, fmt.Errorf("negative source counter")
			}
		}
	}
	return s, nil
}

// RecordAbsolute preserves lifetime totals over process epochs and counter
// resets. An epoch change may have lost unsampled traffic; that uncertainty
// stays recorded and must never be relabelled as complete by a later sample.
// The caller serializes writers (coreinstall uses an interprocess lock).
func RecordAbsolute(source, epoch string, reading map[string]int64) error {
	if source == "" || strings.ContainsAny(source, `/\`) || epoch == "" {
		return fmt.Errorf("source name and engine epoch required")
	}
	path := filepath.Join(Dir, source+".source")
	s, err := readAbsolute(path)
	if os.IsNotExist(err) {
		s = AbsoluteSource{Epoch: epoch, Last: map[string]int64{}, Totals: map[string]int64{}}
	} else if err != nil {
		return err // preserve a damaged ledger; never replace it with zero
	}
	if s.Epoch != epoch {
		s.Incomplete = true
		s.Epoch, s.Last = epoch, map[string]int64{}
	}
	for user, current := range reading {
		account, _, _ := strings.Cut(user, ">>>")
		if current < 0 {
			return fmt.Errorf("negative engine counter")
		}
		previous := s.Last[user]
		delta := current - previous
		if current < previous {
			delta, s.Incomplete = current, true
		}
		if delta > math.MaxInt64-s.Totals[account] {
			return fmt.Errorf("source counter overflow")
		}
		s.Totals[account] += delta
		s.Last[user] = current
	}
	if err := os.MkdirAll(Dir, 0755); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(Dir, ".source-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if d, err := os.Open(Dir); err == nil {
		err = d.Sync()
		_ = d.Close()
		if err != nil {
			return err
		}
	}
	if s.Incomplete {
		return ErrUnsampledTraffic
	}
	return nil
}
