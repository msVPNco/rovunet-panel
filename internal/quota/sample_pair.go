package quota

import (
	"context"
	"database/sql"
)

// ObservePair claims the entire physical traffic domain for a sampled source.
// Both directions and its receipt commit together. This never authorizes I/O.
// "account-total" deliberately forbids mixing native TCP and sampled authority
// for one period until a verified disjoint-domain provisioning contract exists.
func (s *Store) ObservePair(ctx context.Context, k Key, op, source, epoch string, ub, db, up, down, stamp int64) error {
	if !valid(source) || source == "native" || !valid(epoch) || ub < 0 || db < 0 || up < ub || down < db || stamp < 0 {
		return ErrConflict
	}
	return s.change(ctx, k, "observe-pair", op, []any{source, epoch, ub, db, up, down, stamp}, func(ctx context.Context, c *sql.Conn) error {
		if _, e := active(ctx, c, k); e != nil {
			return e
		}
		if e := claimMeter(ctx, c, k, "sample", source); e != nil {
			return e
		}
		if e := observeDirection(ctx, c, k, source, epoch, "up", ub, up, stamp, false); e != nil {
			return e
		}
		return observeDirection(ctx, c, k, source, epoch, "down", db, down, stamp, false)
	})
}
func claimMeter(ctx context.Context, c *sql.Conn, k Key, authority, source string) error {
	_, e := c.ExecContext(ctx, "INSERT OR IGNORE INTO meter_bindings VALUES(?,?,'account-total',?,?)", k.Account, k.Period, authority, source)
	if e != nil {
		return e
	}
	var a, s string
	e = c.QueryRowContext(ctx, "SELECT authority,source FROM meter_bindings WHERE account=? AND period=? AND traffic_domain='account-total'", k.Account, k.Period).Scan(&a, &s)
	if e != nil {
		return e
	}
	if a != authority || s != source {
		return ErrConflict
	}
	return nil
}
