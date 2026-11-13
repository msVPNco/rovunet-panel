package quota

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

type sourceRow struct {
	Source, Epoch, Dir                string
	Base, Absolute, Incomplete, Stamp int64
}
type ticketRow struct {
	ID, Meter, Dir          string
	Fence, Requested, Grant int64
	Actual                  sql.NullInt64
}
type receiptRow struct {
	Kind, ID     string
	Hash, Result []byte
}
type priorOpening struct {
	ID    string
	Hash  []byte
	Value int64
}
type snapshotRows struct {
	Limit    int64
	Openings []priorOpening
	Tickets  []ticketRow
	Sources  []sourceRow
	Receipts []receiptRow
}

// MergeBackup merges the specified active period from a verified, closed
// VACUUM INTO snapshot. The caller fences live producers before recovery.
// expectedDigest must come from the authenticated manifest, outside this DB.
func (s *Store) MergeBackup(ctx context.Context, k Key, dir, expectedDigest string) error {
	if !valid(k.Account) || !valid(k.Period) {
		return ErrConflict
	}
	path := filepath.Join(dir, "ledger.sqlite")
	if e := private(dir, true); e != nil {
		return e
	}
	if e := private(path, false); e != nil {
		return e
	}
	real, e := filepath.EvalSymlinks(dir)
	if e != nil || filepath.Clean(dir) != real {
		return ErrConflict
	}
	for _, suf := range []string{"-wal", "-shm", "-journal"} {
		if _, e := os.Lstat(path + suf); !os.IsNotExist(e) {
			return errors.New("expected a closed self-contained snapshot")
		}
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	h := sha256.New()
	_, e = io.Copy(h, f)
	f.Close()
	if e != nil {
		return e
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != expectedDigest {
		return ErrConflict
	}
	uri := fileURI(path, "ro")
	db, e := sql.Open("sqlite", uri)
	if e != nil {
		return e
	}
	defer db.Close()
	tx, e := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var check string
	if e = tx.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check); e != nil || check != "ok" {
		return errors.New("snapshot integrity failure")
	}
	var violations int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&violations); e != nil || violations != 0 {
		return ErrConflict
	}
	var server, id string
	var version, fence int64
	if e = tx.QueryRowContext(ctx, "SELECT server_id,store_id,schema_version,fence FROM store_meta WHERE singleton=1").Scan(&server, &id, &version, &fence); e != nil || id != s.id || server != s.server || version != 1 {
		return ErrConflict
	}
	data, e := loadSnapshot(ctx, tx, k)
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	return s.change(ctx, k, "backup", expectedDigest, expectedDigest, func(ctx context.Context, c *sql.Conn) error {
		lim, e := active(ctx, c, k)
		if e != nil || lim != data.Limit {
			return ErrConflict
		}
		var localFence int64
		if e = c.QueryRowContext(ctx, "SELECT fence FROM store_meta WHERE singleton=1").Scan(&localFence); e != nil || localFence != fence {
			return ErrConflict
		}
		for _, v := range data.Openings {
			var n int64
			var origin string
			var digest []byte
			e := c.QueryRowContext(ctx, "SELECT origin,proof_hash,bytes FROM opening WHERE account=? AND period=?", k.Account, k.Period).Scan(&origin, &digest, &n)
			if errors.Is(e, sql.ErrNoRows) {
				_, e = c.ExecContext(ctx, "INSERT INTO opening VALUES(?,?,?,?,?)", k.Account, k.Period, v.ID, v.Hash, v.Value)
			} else if e == nil && (origin != v.ID || string(digest) != string(v.Hash) || n != v.Value) {
				return ErrConflict
			}
			if e != nil {
				return e
			}
		}
		for _, v := range data.Tickets {
			var old ticketRow
			e = c.QueryRowContext(ctx, "SELECT meter,direction,owner_fence,requested,granted,actual FROM tickets WHERE account=? AND period=? AND op=?", k.Account, k.Period, v.ID).Scan(&old.Meter, &old.Dir, &old.Fence, &old.Requested, &old.Grant, &old.Actual)
			if errors.Is(e, sql.ErrNoRows) {
				_, e = c.ExecContext(ctx, "INSERT INTO tickets VALUES(?,?,?,?,?,?,?,?,?)", k.Account, k.Period, v.ID, v.Meter, v.Dir, v.Fence, v.Requested, v.Grant, v.Actual)
			} else if e == nil {
				if old.Meter != v.Meter || old.Dir != v.Dir || old.Fence != v.Fence || old.Requested != v.Requested || old.Grant != v.Grant || old.Actual.Valid && v.Actual.Valid && old.Actual.Int64 != v.Actual.Int64 {
					return ErrConflict
				}
				if !old.Actual.Valid && v.Actual.Valid {
					_, e = c.ExecContext(ctx, "UPDATE tickets SET actual=? WHERE account=? AND period=? AND op=?", v.Actual.Int64, k.Account, k.Period, v.ID)
				}
			}
			if e != nil {
				return e
			}
		}
		for _, v := range data.Sources {
			res, e := c.ExecContext(ctx, `INSERT INTO source_epochs VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(account,period,source,epoch,direction) DO UPDATE SET observed=max(observed,excluded.observed),sampled_utc=max(sampled_utc,excluded.sampled_utc),incomplete=max(incomplete,excluded.incomplete) WHERE baseline=excluded.baseline`, k.Account, k.Period, v.Source, v.Epoch, v.Dir, v.Base, v.Absolute, v.Incomplete, v.Stamp)
			if e != nil {
				return e
			}
			n, e := res.RowsAffected()
			if e != nil || n != 1 {
				return ErrConflict
			}
		}
		for _, v := range data.Receipts {
			var old, result []byte
			e := c.QueryRowContext(ctx, "SELECT request_hash,result FROM receipts WHERE account=? AND period=? AND kind=? AND id=?", k.Account, k.Period, v.Kind, v.ID).Scan(&old, &result)
			if e == nil {
				if string(old) != string(v.Hash) || string(result) != string(v.Result) {
					return ErrConflict
				}
				continue
			}
			if !errors.Is(e, sql.ErrNoRows) {
				return e
			}
			var seq int64
			if e = c.QueryRowContext(ctx, "UPDATE periods SET sequence=sequence+1 WHERE account=? AND period=? RETURNING sequence", k.Account, k.Period).Scan(&seq); e != nil {
				return e
			}
			if _, e = c.ExecContext(ctx, "INSERT INTO receipts VALUES(?,?,?,?,?,?,?)", k.Account, k.Period, v.Kind, v.ID, v.Hash, v.Result, seq); e != nil {
				return e
			}
		}
		digest := h.Sum(nil)
		_, e = c.ExecContext(ctx, "INSERT INTO checkpoints VALUES(?,?,?,?,?)", k.Account, k.Period, expectedDigest, digest, digest)
		return e
	})
}
func loadSnapshot(ctx context.Context, tx *sql.Tx, k Key) (snapshotRows, error) {
	var d snapshotRows
	e := tx.QueryRowContext(ctx, "SELECT limit_bytes FROM periods WHERE account=? AND period=? AND status='active' AND limit_kind='finite'", k.Account, k.Period).Scan(&d.Limit)
	if e != nil {
		return d, e
	}
	read := func(query string, fn func(*sql.Rows) error) error {
		rows, e := tx.QueryContext(ctx, query, k.Account, k.Period)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			if e = fn(rows); e != nil {
				return e
			}
		}
		return rows.Err()
	}
	if e = read("SELECT origin,proof_hash,bytes FROM opening WHERE account=? AND period=?", func(r *sql.Rows) error {
		var v priorOpening
		e := r.Scan(&v.ID, &v.Hash, &v.Value)
		d.Openings = append(d.Openings, v)
		return e
	}); e != nil || len(d.Openings) > 1 {
		return d, ErrConflict
	}
	if e = read("SELECT op,meter,direction,owner_fence,requested,granted,actual FROM tickets WHERE account=? AND period=?", func(r *sql.Rows) error {
		var v ticketRow
		e := r.Scan(&v.ID, &v.Meter, &v.Dir, &v.Fence, &v.Requested, &v.Grant, &v.Actual)
		d.Tickets = append(d.Tickets, v)
		return e
	}); e != nil {
		return d, e
	}
	if e = read("SELECT source,epoch,direction,baseline,observed,incomplete,sampled_utc FROM source_epochs WHERE account=? AND period=?", func(r *sql.Rows) error {
		var v sourceRow
		e := r.Scan(&v.Source, &v.Epoch, &v.Dir, &v.Base, &v.Absolute, &v.Incomplete, &v.Stamp)
		d.Sources = append(d.Sources, v)
		return e
	}); e != nil {
		return d, e
	}
	received := map[string]bool{}
	if e = read("SELECT kind,id,request_hash,result FROM receipts WHERE account=? AND period=? ORDER BY seq", func(r *sql.Rows) error {
		var v receiptRow
		e := r.Scan(&v.Kind, &v.ID, &v.Hash, &v.Result)
		if e != nil {
			return e
		}
		d.Receipts = append(d.Receipts, v)
		if v.Kind == "reserve" {
			received[v.ID] = true
		}
		return nil
	}); e != nil {
		return d, e
	}
	for _, t := range d.Tickets {
		if !received[t.ID] {
			return d, ErrConflict
		}
	}
	_, e = amount(ctx, tx, k)
	return d, e
}

func fileURI(path, mode string) string {
	u := url.URL{Scheme: "file", Path: path}
	v := url.Values{"mode": {mode}, "immutable": {"1"}, "_pragma": {"trusted_schema(OFF)", "foreign_keys(ON)"}}
	u.RawQuery = v.Encode()
	return u.String()
}
