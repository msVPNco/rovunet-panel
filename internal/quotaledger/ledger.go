// Package quotaledger is the isolated candidate for durable shared reservations.
// Callers must reserve before transfer and settle only measured transferred bytes.
package quotaledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
)

var ErrBudget = errors.New("quota exhausted")
var ErrPeriod = errors.New("retired or stale period")
var ErrReplay = errors.New("reservation exists; replay grants no further transfer")

type Ticket struct {
	Reserved int64
	Actual   *int64
}

// Confirmed and Floor exclude unsettled liability. Charged includes it.
type Period struct {
	Confirmed int64
	Floor     int64
	Limit     int64
	Charged   int64
	Closed    bool
	Tickets   map[string]Ticket
	Sources   map[string]int64
}
type State struct {
	Version int
	Current map[string]string
	Periods map[string]*Period
}
type Store struct{ Directory string }

func valid(s string) bool { return validIdentifier(s) }
func periodKey(account, period string) string {
	b, _ := json.Marshal([]string{account, period})
	return string(b)
}

// transact serializes processes on a permanent lock inode. A successful mutation
// is acknowledged only after atomic file replacement and parent-directory fsync.
func (s Store) transact(change func(*State) error) error {
	info, err := os.Lstat(s.Directory)
	if err != nil {
		return err
	}
	if !privateDirectory(info) {
		return errors.New("ledger requires a private directory")
	}
	lock, err := os.OpenFile(filepath.Join(s.Directory, "write.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	info, err = lock.Stat()
	if err != nil {
		return err
	}
	if !privateFile(info) {
		return errors.New("invalid quota lock file")
	}

	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	path := filepath.Join(s.Directory, "ledger.json")
	state, err := readState(path, lock)
	if err != nil {
		return err
	}
	if err := change(&state); err != nil {
		return err
	}
	return storeState(path, lock, &state)
}

// Open initializes an account once. Renewal is a compare-and-swap operation.
func (s Store) Open(account, period string, limit, initial int64) error {
	if !valid(account) || !valid(period) || limit < 0 || initial < 0 {
		return errors.New("invalid account parameters")
	}
	return s.transact(func(st *State) error {
		if old, ok := st.Current[account]; ok {
			if old != period {
				return ErrPeriod
			}
			p := st.Periods[periodKey(account, period)]
			if p == nil || p.Closed || p.Limit != limit {
				return ErrPeriod
			}
			return nil
		}
		key := periodKey(account, period)
		if st.Periods[key] != nil {
			return ErrPeriod
		}
		st.Current[account] = period
		st.Periods[key] = &Period{Limit: limit, Charged: initial, Confirmed: initial, Floor: initial, Tickets: map[string]Ticket{}, Sources: map[string]int64{}}
		return nil
	})
}
func current(st *State, account, period string) (*Period, error) {
	p := st.Periods[periodKey(account, period)]
	if st.Current[account] != period || p == nil || p.Closed {
		return nil, ErrPeriod
	}
	return p, nil
}
func (s Store) Reserve(account, period, operation string, bytes int64) error {
	if !valid(operation) || bytes <= 0 {
		return errors.New("invalid reservation")
	}
	return s.transact(func(st *State) error {
		p, err := current(st, account, period)
		if err != nil {
			return err
		}
		if old, ok := p.Tickets[operation]; ok {
			if old.Reserved != bytes {
				return errors.New("operation changed")
			}
			return ErrReplay
		}
		if p.Charged > p.Limit || bytes > p.Limit-p.Charged {
			return ErrBudget
		}
		p.Charged += bytes
		p.Tickets[operation] = Ticket{Reserved: bytes}
		return nil
	})
}
func (s Store) Settle(account, period, operation string, actual int64) error {
	return s.transact(func(st *State) error {
		// Settlement from an old period stays in that old period after renewal.
		p := st.Periods[periodKey(account, period)]
		if p == nil {
			return ErrPeriod
		}
		old, ok := p.Tickets[operation]
		if !ok || actual < 0 || actual > old.Reserved {
			return errors.New("invalid settlement")
		}
		if old.Actual != nil {
			if *old.Actual != actual {
				return errors.New("settlement changed")
			}
			return nil
		}
		p.Charged -= old.Reserved - actual
		p.Confirmed += actual
		old.Actual = &actual
		p.Tickets[operation] = old
		return nil
	})
}

// Observe imports monotonic absolute bytes for one period-scoped source/epoch.
// It accounts sampled engines but cannot pre-authorize their traffic.
func (s Store) Observe(account, period, source string, absolute int64) error {
	if !valid(source) || absolute < 0 {
		return errors.New("invalid source observation")
	}
	return s.transact(func(st *State) error {
		p, err := current(st, account, period)
		if err != nil {
			return err
		}
		old := p.Sources[source]
		if absolute < old {
			return errors.New("source reset requires a new authenticated epoch")
		}
		delta := absolute - old
		if delta > math.MaxInt64-p.Charged {
			return errors.New("quota overflow")
		}
		p.Charged += delta
		p.Confirmed += delta
		p.Sources[source] = absolute
		return nil
	})
}
func (s Store) Renew(account, expected, next string, limit int64) error {
	if !valid(next) || limit < 0 {
		return errors.New("invalid renewal")
	}
	return s.transact(func(st *State) error {
		p, err := current(st, account, expected)
		if err != nil {
			return err
		}
		key := periodKey(account, next)
		if st.Periods[key] != nil {
			return ErrPeriod
		}
		p.Closed = true
		st.Current[account] = next
		st.Periods[key] = &Period{Limit: limit, Tickets: map[string]Ticket{}, Sources: map[string]int64{}}
		return nil
	})
}

// RaiseFloor accepts a verified same-period confirmed-only floor. It excludes
// unsettled reservations and overlapping unanchored source observations. Such
// ambiguous backups require operation-aware reconciliation before this API.
// Settlements after this checkpoint add confirmed bytes exactly once.
func (s Store) RaiseFloor(account, period string, floor int64) error {
	if floor < 0 {
		return errors.New("invalid restore floor")
	}
	return s.transact(func(st *State) error {
		p, err := current(st, account, period)
		if err != nil {
			return err
		}
		if floor > p.Confirmed {
			increased, err := checkedAdd(p.Charged, floor-p.Confirmed)
			if err != nil {
				return err
			}
			p.Charged = increased
			p.Confirmed = floor
		}
		if floor > p.Floor {
			p.Floor = floor
		}
		return nil
	})
}
func (s Store) Read(account, period string) (Period, error) {
	var result Period
	err := s.transact(func(st *State) error {
		p := st.Periods[periodKey(account, period)]
		if p == nil {
			return fmt.Errorf("unknown period")
		}
		result = *p
		return nil
	})
	return result, err
}
