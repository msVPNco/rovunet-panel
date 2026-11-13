package quota

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
)

// CarryPeriod transfers exclusive remaining budget to a fresh local period.
// Caller quiesces old sessions before invoking it, then publishes config via
// CAS. A config-save failure leaves old admission closed and needs forward
// recovery; it never opens both budgets. Unknown pending prevents the cutover.
func (s *Store) CarryPeriod(ctx context.Context, k Key, op, next string) error {
	return s.CarryPeriodWithExpiry(ctx, k, op, next, nil)
}
func (s *Store) CarryPeriodWithExpiry(ctx context.Context, k Key, op, next string, expires *int64) error {
	if !valid(next) || next == k.Period {
		return ErrConflict
	}
	return s.change(ctx, k, "carry-period", op, []any{next, expires}, func(ctx context.Context, c *sql.Conn) error {
		limit, e := active(ctx, c, k)
		if e != nil {
			return e
		}
		a, e := amount(ctx, c, k)
		if e != nil {
			return e
		}
		if a.Pending != 0 || a.Confirmed > limit {
			return ErrConflict
		}
		if _, e = c.ExecContext(ctx, "UPDATE periods SET status='closed' WHERE account=? AND period=?", k.Account, k.Period); e != nil {
			return e
		}
		kind := "none"
		if expires != nil {
			kind = "at"
		}
		if _, e = c.ExecContext(ctx, "INSERT INTO periods VALUES(?,?,'active','finite',?,?,?,0)", k.Account, next, limit, kind, expires); e != nil {
			return e
		}
		proof := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%s/%d", s.server, k.Account, k.Period, op, a.Confirmed)))
		_, e = c.ExecContext(ctx, "INSERT INTO opening VALUES(?,?,?,?,?)", k.Account, next, "carry:"+op, proof[:], a.Confirmed)
		return e
	})
}
