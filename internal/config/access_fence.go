package config

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"
	"time"
)

// A finite TCP session holds live shared until both socket pumps have stopped
// and settlement has been attempted. A config writer closes entry first and
// waits for live exclusive before publishing the change or acknowledging it.
// Separate processes open separate file descriptions; lock files are never
// unlinked. The kernel releases their locks on process death.
type AccessLease struct{ gate, live *os.File }

func accessLock(path, user, kind string) (*os.File, error) {
	canonical, e := filepath.EvalSymlinks(path)
	if e != nil || canonical != path {
		return nil, errors.New("access fencing requires the canonical absolute config path")
	}
	root := path + ".access"
	if err := os.Mkdir(root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	st, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 || st.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return nil, errors.New("invalid access fence directory")
	}
	file := filepath.Join(root, fmt.Sprintf("%x.%s", sha256.Sum256([]byte(user)), kind))
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	st, err = f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || st.Sys().(*syscall.Stat_t).Nlink != 1 || st.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		f.Close()
		return nil, errors.New("invalid access fence file")
	}
	return f, nil
}
func tryAccess(f *os.File, mode int) error { return syscall.Flock(int(f.Fd()), mode|syscall.LOCK_NB) }
func AdmitFinite(path, user string) (*AccessLease, error) {
	gate, e := accessLock(path, user, "gate")
	if e != nil {
		return nil, e
	}
	if e = tryAccess(gate, syscall.LOCK_SH); e != nil {
		gate.Close()
		return nil, e
	}
	live, e := accessLock(path, user, "live")
	if e == nil {
		e = tryAccess(live, syscall.LOCK_SH)
	}
	if e != nil {
		if live != nil {
			live.Close()
		}
		gate.Close()
		return nil, e
	}
	if e = syscall.Flock(int(gate.Fd()), syscall.LOCK_UN); e != nil {
		live.Close()
		gate.Close()
		return nil, e
	}
	return &AccessLease{gate, live}, nil
}

// Check uses a fresh description; concurrent watcher and writer checks must
// never unlock each other's shared lock.
func (l *AccessLease) Check() error {
	f, e := os.OpenFile(l.gate.Name(), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	return tryAccess(f, syscall.LOCK_SH)
}
func (l *AccessLease) Close() { l.live.Close(); l.gate.Close() }
func exclusive(f *os.File, until time.Time) error {
	for {
		e := tryAccess(f, syscall.LOCK_EX)
		if !errors.Is(e, syscall.EWOULDBLOCK) {
			return e
		}
		if time.Now().After(until) {
			return errors.New("access change not acknowledged: sessions did not drain")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func fencedSave(path string, previous []byte, next *Config, commit func() error) error {
	if len(previous) == 0 {
		return commit()
	}
	var old Config
	if e := json.Unmarshal(previous, &old); e != nil {
		return e
	}
	names := map[string]bool{}
	byName := map[string]UserConfig{}
	for _, u := range next.Users {
		byName[u.Username] = u
	}
	prior := append([]UserConfig(nil), old.Users...)
	for _, a := range old.Archived {
		if old.GetUser(a.Username) == nil {
			prior = append(prior, a.UserConfig)
		}
	}
	for _, u := range prior {
		v, ok := byName[u.Username]
		if ok && u.Password != v.Password && u.SubToken != "" && v.SubToken == u.SubToken {
			return errors.New("password change requires a new private subscription token")
		}
		if u.QuotaGB > 0 && (!ok || old.ServerID != next.ServerID || old.RecoverySealed != next.RecoverySealed || u.Password != v.Password || u.AccountID != v.AccountID || u.PeriodID != v.PeriodID || u.ExpiresAt != v.ExpiresAt || u.QuotaGB != v.QuotaGB || !reflect.DeepEqual(u.SQLiteMeter, v.SQLiteMeter)) {
			names[u.Username] = true
		}
	}
	ordered := make([]string, 0, len(names))
	for n := range names {
		ordered = append(ordered, n)
	}
	sort.Strings(ordered)
	deadline := time.Now().Add(10 * time.Second)
	for _, kind := range []string{"gate", "live"} {
		for _, name := range ordered {
			f, e := accessLock(path, name, kind)
			if e != nil {
				return e
			}
			defer f.Close()
			if e = exclusive(f, deadline); e != nil {
				return e
			}
		}
	}
	return commit()
}
