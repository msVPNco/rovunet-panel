package quotaledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

const initializedMarker = "rovunet-quota-store-v2\n"

func validIdentifier(value string) bool {
	return value != "" && len(value) <= 200 && strings.TrimSpace(value) == value && utf8.ValidString(value)
}

func requiredObject(data []byte, target any, names []string, nullable string) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return err
	}
	if len(members) != len(names) {
		return errors.New("missing or unexpected quota members")
	}
	for _, name := range names {
		value, ok := members[name]
		if !ok || (name != nullable && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return fmt.Errorf("missing or null quota member %s", name)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (state *State) UnmarshalJSON(data []byte) error {
	type plain State
	return requiredObject(data, (*plain)(state), []string{"Version", "Current", "Periods"}, "")
}
func (period *Period) UnmarshalJSON(data []byte) error {
	type plain Period
	var raw struct{ Sources map[string]json.RawMessage }
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, amount := range raw.Sources {
		if bytes.Equal(bytes.TrimSpace(amount), []byte("null")) {
			return errors.New("null counter source amount")
		}
	}
	return requiredObject(data, (*plain)(period), []string{"Limit", "Charged", "Confirmed", "Floor", "Closed", "Tickets", "Sources"}, "")
}
func (ticket *Ticket) UnmarshalJSON(data []byte) error {
	type plain Ticket
	return requiredObject(data, (*plain)(ticket), []string{"Reserved", "Actual"}, "Actual")
}

func checkJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("invalid JSON encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				token, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := token.(string)
				if !ok || seen[key] {
					return errors.New("duplicate JSON member")
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing quota data")
	}
	return nil
}

func checkedAdd(total, amount int64) (int64, error) {
	if total < 0 || amount < 0 || amount > math.MaxInt64-total {
		return 0, errors.New("negative or overflowing quota amount")
	}
	return total + amount, nil
}

func validateState(state *State) error {
	if state.Version != 2 || state.Current == nil || state.Periods == nil {
		return errors.New("invalid quota schema; explicit compatible recovery required")
	}
	for account, period := range state.Current {
		if !valid(account) || !valid(period) {
			return errors.New("invalid current account mapping")
		}
		current := state.Periods[periodKey(account, period)]
		if current == nil || current.Closed {
			return errors.New("current period missing or closed")
		}
	}
	for key, period := range state.Periods {
		var pair []string
		if json.Unmarshal([]byte(key), &pair) != nil || len(pair) != 2 || !valid(pair[0]) || !valid(pair[1]) || periodKey(pair[0], pair[1]) != key {
			return errors.New("invalid period key")
		}
		if period == nil || period.Limit < 0 || period.Confirmed < 0 || period.Floor < 0 || period.Floor > period.Confirmed || period.Tickets == nil || period.Sources == nil {
			return errors.New("invalid quota period")
		}
		current, exists := state.Current[pair[0]]
		if !exists || (!period.Closed && current != pair[1]) {
			return errors.New("inconsistent account/period mapping")
		}
		var pending, accounted int64
		for operation, ticket := range period.Tickets {
			if !valid(operation) || ticket.Reserved <= 0 {
				return errors.New("invalid quota ticket")
			}
			var err error
			if ticket.Actual == nil {
				pending, err = checkedAdd(pending, ticket.Reserved)
			} else {
				if *ticket.Actual < 0 || *ticket.Actual > ticket.Reserved {
					return errors.New("invalid settled amount")
				}
				accounted, err = checkedAdd(accounted, *ticket.Actual)
			}
			if err != nil {
				return err
			}
		}
		for source, amount := range period.Sources {
			if !valid(source) {
				return errors.New("invalid counter source")
			}
			var err error
			accounted, err = checkedAdd(accounted, amount)
			if err != nil {
				return err
			}
		}
		charged, err := checkedAdd(period.Confirmed, pending)
		if err != nil || charged != period.Charged || accounted > period.Confirmed {
			return errors.New("inconsistent quota amounts")
		}
	}
	return nil
}

func privateDirectory(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	return info.IsDir() && info.Mode().Perm()&0077 == 0 && ok && owner.Uid == uint32(os.Geteuid())
}
func privateFile(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	return info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0 && ok && owner.Uid == uint32(os.Geteuid()) && owner.Nlink == 1
}

func readState(path string, lock *os.File) (State, error) {
	var state State
	marker, err := io.ReadAll(lock)
	if err != nil {
		return state, err
	}
	if len(marker) != 0 && string(marker) != initializedMarker {
		return state, errors.New("invalid store initialization marker")
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		if len(marker) != 0 {
			return state, errors.New("initialized store is missing; recovery required")
		}
		return State{Version: 2, Current: map[string]string{}, Periods: map[string]*Period{}}, nil
	}
	if err != nil {
		return state, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return state, err
	}
	if !privateFile(info) {
		return state, errors.New("invalid quota data file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return state, err
	}
	if err := checkJSON(data); err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, validateState(&state)
}

func storeState(path string, lock *os.File, state *State) error {
	if err := validateState(state); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	info, err := lock.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		if _, err := lock.WriteAt([]byte(initializedMarker), 0); err != nil {
			return err
		}
		if err := lock.Sync(); err != nil {
			return err
		}
		if err := safefile.SyncDir(filepath.Dir(path)); err != nil {
			return err
		}
	}
	return safefile.Write(path, data, 0600)
}
