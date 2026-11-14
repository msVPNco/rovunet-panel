// Package quota contains the experimental per-server SQLite ledger.
package quota

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/safefile"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var ErrBudget = errors.New("quota capacity unavailable")
var ErrReplay = errors.New("operation already committed; no new grant")
var ErrConflict = errors.New("quota identity, period or receipt conflict")

type Key struct{ Account, Period string }
type Store struct {
	db         *sql.DB
	policyDB   *sql.DB
	server, id string
	directory  string
}
type Amount struct{ Confirmed, Pending int64 }
type Checkpoint struct {
	Server, Store, Account, Period, ID, OpeningOrigin, Proof string
	Opening                                                  int64
}

func valid(s string) bool {
	return len(s) > 0 && len(s) <= 200 && strings.TrimSpace(s) == s && !strings.ContainsRune(s, 0)
}
func add(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > math.MaxInt64-b {
		return 0, errors.New("invalid quota total")
	}
	return a + b, nil
}
func private(path string, dir bool) error {
	i, e := os.Lstat(path)
	if e != nil {
		return e
	}
	st, ok := i.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) || i.Mode().Perm()&0077 != 0 || i.Mode()&os.ModeSymlink != 0 || dir != i.IsDir() || (!dir && (!i.Mode().IsRegular() || st.Nlink != 1)) {
		return errors.New("invalid private quota path")
	}
	return nil
}

// Create requires a fresh directory. An interrupted bootstrap cannot silently reinitialize it.
func Create(ctx context.Context, dir, server, id string) (*Store, error) {
	if !valid(server) || !valid(id) {
		return nil, ErrConflict
	}
	if e := os.Mkdir(dir, 0700); e != nil {
		return nil, e
	}
	if e := safefile.Write(filepath.Join(dir, "bootstrap.pending"), []byte(id), 0600); e != nil {
		return nil, e
	}
	path := filepath.Join(dir, "ledger.sqlite")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	s, e := connect(ctx, dir, server, id)
	if e != nil {
		return nil, e
	}
	if _, e = s.db.ExecContext(ctx, schema); e == nil {
		_, e = s.db.ExecContext(ctx, "INSERT INTO store_meta VALUES(1,1,?,?,1)", id, server)
	}
	if e == nil {
		e = safefile.SyncDir(dir)
	}
	if e == nil {
		e = safefile.SyncDir(filepath.Dir(dir))
	}
	if e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}
func Open(ctx context.Context, dir, server, id string) (*Store, error) {
	if _, e := os.Lstat(filepath.Join(dir, "bootstrap.pending")); e == nil || !os.IsNotExist(e) {
		return nil, errors.New("quota bootstrap incomplete")
	}
	s, e := connect(ctx, dir, server, id)
	if e != nil {
		return nil, e
	}
	var dbID, dbServer string
	var rev, userRev int
	e = s.db.QueryRowContext(ctx, "SELECT store_id,server_id,schema_version FROM store_meta WHERE singleton=1").Scan(&dbID, &dbServer, &rev)
	if e == nil {
		e = s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userRev)
	}
	if e != nil || dbID != id || dbServer != server || rev != 1 || userRev != 1 {
		s.Close()
		return nil, ErrConflict
	}
	return s, nil
}
func connect(ctx context.Context, dir, server, id string) (*Store, error) {
	if !valid(server) || !valid(id) {
		return nil, ErrConflict
	}
	path, e := filepath.Abs(filepath.Join(dir, "ledger.sqlite"))
	if e != nil {
		return nil, e
	}
	real, e := filepath.EvalSymlinks(filepath.Dir(path))
	if e != nil || real != filepath.Dir(path) {
		return nil, errors.New("invalid quota directory lineage")
	}
	if e = secureAncestors(path); e != nil {
		return nil, e
	}
	if e = private(filepath.Dir(path), true); e != nil {
		return nil, e
	}
	if e = private(path, false); e != nil {
		return nil, e
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if e = private(path+suffix, false); e != nil && !os.IsNotExist(e) {
			return nil, e
		}
	}
	address := url.URL{Scheme: "file", Path: path}
	v := url.Values{"mode": {"rw"}, "_txlock": {"immediate"}, "_pragma": {"busy_timeout(250)", "foreign_keys(ON)", "trusted_schema(OFF)", "synchronous(FULL)", "journal_mode(WAL)"}}
	address.RawQuery = v.Encode()
	db, e := sql.Open("sqlite", address.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, server: server, id: id, directory: filepath.Dir(path)}
	connectionCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	var c *sql.Conn
	release, e := acquireWrite(connectionCtx, s.directory, false)
	if e != nil {
		db.Close()
		return nil, e
	}
	defer release()
	e = waitBusy(connectionCtx, func() error { var err error; c, err = db.Conn(connectionCtx); return err })
	if e != nil {
		db.Close()
		return nil, e
	}
	e = verify(ctx, c)
	c.Close()
	if e != nil {
		db.Close()
		return nil, e
	}
	// WAL policy reads must not queue behind this process's sole writer
	// connection (including a slow FULL fsync). They observe committed state
	// only. Capacity is still granted exclusively by the serialized writer.
	readAddress := address
	readValues := url.Values{"mode": {"ro"}, "_pragma": {"query_only(ON)", "busy_timeout(250)", "foreign_keys(ON)", "trusted_schema(OFF)"}}
	readAddress.RawQuery = readValues.Encode()
	s.policyDB, e = sql.Open("sqlite", readAddress.String())
	if e != nil {
		db.Close()
		return nil, e
	}
	s.policyDB.SetMaxOpenConns(2)
	s.policyDB.SetMaxIdleConns(2)
	return s, nil
}
func verify(ctx context.Context, c *sql.Conn) error {
	for _, setting := range []struct{ p, want string }{{"journal_mode", "wal"}, {"synchronous", "2"}, {"foreign_keys", "1"}, {"busy_timeout", "250"}, {"trusted_schema", "0"}} {
		var v string
		if e := c.QueryRowContext(ctx, "PRAGMA "+setting.p).Scan(&v); e != nil {
			return e
		}
		if v != setting.want {
			return fmt.Errorf("ineffective pragma %s", setting.p)
		}
	}
	var ver string
	if e := c.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&ver); e != nil {
		return e
	}
	parts := strings.Split(ver, ".")
	if len(parts) != 3 {
		return errors.New("unknown SQLite version")
	}
	var n [3]int
	for i, p := range parts {
		x, e := strconv.Atoi(p)
		if e != nil {
			return e
		}
		n[i] = x
	}
	if !(n[0] > 3 || n[0] == 3 && (n[1] > 51 || n[1] == 51 && n[2] >= 3)) {
		return errors.New("embedded SQLite WAL fix not verified")
	}
	return nil
}
func (s *Store) Close() error { return errors.Join(s.policyDB.Close(), s.db.Close()) }
func (s *Store) write(ctx context.Context, fn func(context.Context, *sql.Conn) error) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	release, e := acquireWrite(ctx, s.directory, ctx.Value(writeClass{}) == "data")
	if e != nil {
		return e
	}
	defer release()
	c, e := s.db.Conn(ctx)
	if e != nil {
		return e
	}
	defer c.Close()
	if e = verify(ctx, c); e != nil {
		return e
	}
	if e = waitBusy(ctx, func() error { _, err := c.ExecContext(ctx, "BEGIN IMMEDIATE"); return err }); e != nil {
		return e
	}
	committed := false
	defer func() {
		if !committed {
			clean, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			if _, r := c.ExecContext(clean, "ROLLBACK"); r != nil {
				_ = c.Raw(func(any) error { return driver.ErrBadConn })
			}
		}
	}()
	if e = fn(ctx, c); e != nil {
		return e
	}
	_, e = c.ExecContext(ctx, "COMMIT")
	committed = e == nil
	return e
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func amount(ctx context.Context, c queryer, k Key) (Amount, error) {
	var b, t, x, p int64
	if e := c.QueryRowContext(ctx, "SELECT opening_bytes,settled_bytes,sampled_bytes,pending_bytes FROM quantities WHERE account=? AND period=?", k.Account, k.Period).Scan(&b, &t, &x, &p); e != nil {
		return Amount{}, e
	}
	n, e := add(b, t)
	if e != nil {
		return Amount{}, e
	}
	n, e = add(n, x)
	if e != nil {
		return Amount{}, e
	}
	_, e = add(n, p)
	return Amount{n, p}, e
}
func (s *Store) Read(ctx context.Context, k Key) (Amount, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return amount(ctx, s.policyDB, k)
}
func (s *Store) NewPeriod(ctx context.Context, k Key, limit int64) error {
	if !valid(k.Account) || !valid(k.Period) || limit < 0 {
		return ErrConflict
	}
	return s.write(ctx, func(ctx context.Context, c *sql.Conn) error {
		var old int64
		var state string
		e := c.QueryRowContext(ctx, "SELECT limit_bytes,status FROM periods WHERE account=? AND period=?", k.Account, k.Period).Scan(&old, &state)
		if e == nil {
			if old == limit && state == "active" {
				return nil
			}
			return ErrConflict
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		_, e = c.ExecContext(ctx, "INSERT INTO periods VALUES(?,?,'active','finite',?,'none',NULL,0)", k.Account, k.Period, limit)
		return e
	})
}

// change records the operation and mutation in one transaction. A reserve replay never authorizes new I/O.
func (s *Store) change(ctx context.Context, k Key, kind, id string, payload any, mutate func(context.Context, *sql.Conn) error) error {
	if !valid(k.Account) || !valid(k.Period) || !valid(id) {
		return ErrConflict
	}
	body, e := json.Marshal([]any{1, s.server, s.id, k, kind, id, payload})
	if e != nil {
		return e
	}
	sum := sha256.Sum256(body)
	if kind == "reserve" || kind == "settle" {
		ctx = context.WithValue(ctx, writeClass{}, "data")
	}
	return s.write(ctx, func(ctx context.Context, c *sql.Conn) error {
		var saved []byte
		e := c.QueryRowContext(ctx, "SELECT request_hash FROM receipts WHERE account=? AND period=? AND kind=? AND id=?", k.Account, k.Period, kind, id).Scan(&saved)
		if e == nil {
			if string(saved) != string(sum[:]) {
				return ErrConflict
			}
			if kind == "reserve" {
				return ErrReplay
			}
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if e = mutate(ctx, c); e != nil {
			return e
		}
		a, e := amount(ctx, c, k)
		if e != nil {
			return e
		}
		if e = noteLimit(ctx, c, k, a); e != nil {
			return e
		}
		var seq int64
		if e = c.QueryRowContext(ctx, "UPDATE periods SET sequence=sequence+1 WHERE account=? AND period=? AND sequence<? RETURNING sequence", k.Account, k.Period, int64(math.MaxInt64)).Scan(&seq); e != nil {
			return e
		}
		_, e = c.ExecContext(ctx, "INSERT INTO receipts VALUES(?,?,?,?,?,?,?)", k.Account, k.Period, kind, id, sum[:], []byte("committed"), seq)
		return e
	})
}
func active(ctx context.Context, c *sql.Conn, k Key) (int64, error) {
	var lim int64
	e := c.QueryRowContext(ctx, "SELECT limit_bytes FROM periods WHERE account=? AND period=? AND status='active' AND limit_kind='finite' AND (expires_kind='none' OR (expires_kind='at' AND expires_utc>unixepoch()))", k.Account, k.Period).Scan(&lim)
	return lim, e
}
func (s *Store) Reserve(ctx context.Context, k Key, id, dir string, n int64) error {
	return s.reserve(ctx, k, id, dir, n, false)
}
func (s *Store) ReserveNative(ctx context.Context, k Key, id, dir string, n int64) error {
	return s.reserve(ctx, k, id, dir, n, true)
}
func (s *Store) reserve(ctx context.Context, k Key, id, dir string, n int64, bound bool) error {
	if (dir != "up" && dir != "down") || n <= 0 {
		return ErrConflict
	}
	return s.change(ctx, k, "reserve", id, []any{dir, n}, func(ctx context.Context, c *sql.Conn) error {
		if bound {
			if e := claimMeter(ctx, c, k, "ticket", "native"); e != nil {
				return e
			}
		}
		lim, e := active(ctx, c, k)
		if e != nil {
			return e
		}
		a, e := amount(ctx, c, k)
		if e != nil {
			return e
		}
		if a.Confirmed > lim || a.Pending > lim-a.Confirmed || n > lim-a.Confirmed-a.Pending {
			return ErrBudget
		}
		_, e = c.ExecContext(ctx, "INSERT INTO tickets VALUES(?,?,?,'native',?,(SELECT fence FROM store_meta WHERE singleton=1),?,?,NULL)", k.Account, k.Period, id, dir, n, n)
		return e
	})
}
func (s *Store) Settle(ctx context.Context, k Key, id string, n int64) error {
	if n < 0 {
		return ErrConflict
	}
	return s.change(ctx, k, "settle", id, n, func(ctx context.Context, c *sql.Conn) error {
		result, e := c.ExecContext(ctx, "UPDATE tickets SET actual=? WHERE account=? AND period=? AND op=? AND actual IS NULL AND granted>=?", n, k.Account, k.Period, id, n)
		if e != nil {
			return e
		}
		rows, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if rows != 1 {
			var a int64
			if e = c.QueryRowContext(ctx, "SELECT actual FROM tickets WHERE account=? AND period=? AND op=?", k.Account, k.Period, id).Scan(&a); e != nil || a != n {
				return ErrConflict
			}
		}
		return nil
	})
}

// ImportOpening accepts one verified historical segment ending before the first
// locally metered byte. The caller verifies provenance and disjoint coverage.
func (s *Store) ImportOpening(ctx context.Context, p Checkpoint) error {
	if p.Server != s.server || p.Store != s.id || p.Opening < 0 || !valid(p.OpeningOrigin) || !valid(p.Proof) {
		return ErrConflict
	}
	k := Key{p.Account, p.Period}
	hash := sha256.Sum256([]byte(p.Proof))
	return s.change(ctx, k, "opening", p.ID, p, func(ctx context.Context, c *sql.Conn) error {
		if _, e := active(ctx, c, k); e != nil {
			return e
		}
		var origin string
		var n int64
		var proof []byte
		e := c.QueryRowContext(ctx, "SELECT origin,bytes,proof_hash FROM opening WHERE account=? AND period=?", k.Account, k.Period).Scan(&origin, &n, &proof)
		if e == nil {
			if origin != p.OpeningOrigin || n != p.Opening || string(proof) != string(hash[:]) {
				return ErrConflict
			}
		} else if errors.Is(e, sql.ErrNoRows) {
			if _, e = c.ExecContext(ctx, "INSERT INTO opening VALUES(?,?,?,?,?)", k.Account, k.Period, p.OpeningOrigin, hash[:], p.Opening); e != nil {
				return e
			}
		} else {
			return e
		}
		b, e := json.Marshal(p)
		if e != nil {
			return e
		}
		digest := sha256.Sum256(b)
		_, e = c.ExecContext(ctx, "INSERT INTO checkpoints VALUES(?,?,?,?,?)", k.Account, k.Period, p.ID, hash[:], digest[:])
		return e
	})
}

func (s *Store) MatchesBinding(dir, server, id string) bool {
	return s != nil && filepath.IsAbs(dir) && filepath.Clean(dir) == s.directory && server == s.server && id == s.id
}
