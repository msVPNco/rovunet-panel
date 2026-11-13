package quota

import (
	"context"
	"database/sql"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"os"
	"path/filepath"
)

func (s *Store) Renew(ctx context.Context, k Key, op, next string, limit int64) error {
	if !valid(next) || k.Period == next || limit < 0 {
		return ErrConflict
	}
	return s.change(ctx, k, "renew", op, []any{next, limit}, func(ctx context.Context, c *sql.Conn) error {
		r, e := c.ExecContext(ctx, "UPDATE periods SET status='closed' WHERE account=? AND period=? AND status='active'", k.Account, k.Period)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return ErrConflict
		}
		_, e = c.ExecContext(ctx, "INSERT INTO periods VALUES(?,?,'active','finite',?,'none',NULL,0)", k.Account, next, limit)
		return e
	})
}
func (s *Store) Observe(ctx context.Context, k Key, op, source, epoch, dir string, baseline, absolute, stamp int64, gap bool) error {
	if !valid(source) || source == "native" || !valid(epoch) || (dir != "up" && dir != "down") || baseline < 0 || absolute < baseline || stamp < 0 {
		return ErrConflict
	}
	return s.change(ctx, k, "observe", op, []any{source, epoch, dir, baseline, absolute, stamp, gap}, func(ctx context.Context, c *sql.Conn) error {
		if _, e := active(ctx, c, k); e != nil {
			return e
		}
		return observeDirection(ctx, c, k, source, epoch, dir, baseline, absolute, stamp, gap)
	})
}

// Backup creates a self-contained snapshot in a new private directory.
func (s *Store) Backup(ctx context.Context, destination string) error {
	if e := os.Mkdir(destination, 0700); e != nil {
		return e
	}
	path := filepath.Join(destination, "ledger.sqlite")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if _, e = s.db.ExecContext(ctx, "VACUUM INTO ?", path); e != nil {
		return e
	}
	f, e = os.Open(path)
	if e != nil {
		return e
	}
	e = f.Sync()
	f.Close()
	if e != nil {
		return e
	}
	if e = safefile.SyncDir(destination); e != nil {
		return e
	}
	return safefile.SyncDir(filepath.Dir(destination))
}

// SettleProof is an authenticated operation receipt normalized by the trusted
// importer. An open row supplies no new transfer authority on recovery.
type TicketProof struct {
	ID, Direction string
	Grant         int64
	Actual        *int64
}
type TicketSnapshot struct {
	Server, Store, Account, Period, ID string
	Tickets                            []TicketProof
}

// MergeTickets is called with writers fenced. The first implementation accepts
// only tickets already issued by this ledger; unfamiliar issuers/rows require
// a verified offline migration. Existing settled values must agree.
func (s *Store) MergeTickets(ctx context.Context, snap TicketSnapshot) error {
	if snap.Server != s.server || snap.Store != s.id {
		return ErrConflict
	}
	k := Key{snap.Account, snap.Period}
	return s.change(ctx, k, "ticket-checkpoint", snap.ID, snap, func(ctx context.Context, c *sql.Conn) error {
		if _, e := active(ctx, c, k); e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, row := range snap.Tickets {
			if seen[row.ID] || !valid(row.ID) || row.Grant <= 0 {
				return ErrConflict
			}
			seen[row.ID] = true
			var actual sql.NullInt64
			var grant int64
			var d string
			if e := c.QueryRowContext(ctx, "SELECT granted,direction,actual FROM tickets WHERE account=? AND period=? AND op=?", k.Account, k.Period, row.ID).Scan(&grant, &d, &actual); e != nil {
				return e
			}
			if grant != row.Grant || d != row.Direction {
				return ErrConflict
			}
			if row.Actual == nil {
				continue
			}
			n := *row.Actual
			if n < 0 || n > grant || (actual.Valid && actual.Int64 != n) {
				return ErrConflict
			}
			if !actual.Valid {
				if _, e := c.ExecContext(ctx, "UPDATE tickets SET actual=? WHERE account=? AND period=? AND op=?", n, k.Account, k.Period, row.ID); e != nil {
					return e
				}
			}
		}
		return nil
	})
}

// FinishBootstrap is explicit after every manifest opening has been committed.
func (s *Store) FinishBootstrap(dir string) error {
	if filepath.Clean(dir) != s.directory {
		return ErrConflict
	}
	path := filepath.Join(dir, "bootstrap.pending")
	marker, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	if string(marker) != s.id {
		return ErrConflict
	}
	if e = os.Remove(path); e != nil {
		return e
	}
	return safefile.SyncDir(dir)
}

func observeDirection(ctx context.Context, c *sql.Conn, k Key, source, epoch, dir string, baseline, absolute, stamp int64, gap bool) error {
	var prev, base, priorStamp int64
	e := c.QueryRowContext(ctx, "SELECT baseline,observed,sampled_utc FROM source_epochs WHERE account=? AND period=? AND source=? AND epoch=? AND direction=?", k.Account, k.Period, source, epoch, dir).Scan(&base, &prev, &priorStamp)
	if errors.Is(e, sql.ErrNoRows) {
		_, e = c.ExecContext(ctx, "INSERT INTO source_epochs VALUES(?,?,?,?,?,?,?,?,?)", k.Account, k.Period, source, epoch, dir, baseline, absolute, gap, stamp)
		return e
	}
	if e != nil {
		return e
	}
	if base != baseline || absolute < prev || stamp < priorStamp {
		return ErrConflict
	}
	_, e = c.ExecContext(ctx, "UPDATE source_epochs SET observed=?,sampled_utc=?,incomplete=MAX(incomplete,?) WHERE account=? AND period=? AND source=? AND epoch=? AND direction=?", absolute, stamp, gap, k.Account, k.Period, source, epoch, dir)
	return e
}
