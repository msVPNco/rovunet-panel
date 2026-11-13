package quota

import (
	"context"
	"database/sql"
	"errors"
	"golang.org/x/sys/unix"
)

func monotonicNanos() (int64, error) {
	var t unix.Timespec
	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &t)
	return t.Nano(), err
}
func noteLimit(ctx context.Context, c *sql.Conn, k Key, a Amount) error {
	var lim int64
	err := c.QueryRowContext(ctx, "SELECT limit_bytes FROM periods WHERE account=? AND period=? AND limit_kind='finite'", k.Account, k.Period).Scan(&lim)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil || a.Confirmed < lim {
		return err
	}
	stamp, e := monotonicNanos()
	if e != nil {
		return e
	}
	_, e = c.ExecContext(ctx, "INSERT OR IGNORE INTO limit_observations VALUES(?,?,?,?)", k.Account, k.Period, stamp, a.Confirmed)
	return e
}
