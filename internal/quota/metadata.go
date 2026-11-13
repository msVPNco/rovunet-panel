package quota

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"
)

type Metadata struct {
	Limit, Confirmed, Pending, Sequence int64
	State                               string
	SampledAt                           *int64
	Upload, Download                    *int64
}

// Direction totals are complete only if the opening balance is zero and the
// corresponding measurement has been seen without an unresolved source gap.
func (s *Store) Metadata(ctx context.Context, k Key) (Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	tx, e := s.db.Conn(ctx)
	if e != nil {
		return Metadata{}, e
	}
	defer tx.Close()
	if _, e = tx.ExecContext(ctx, "BEGIN DEFERRED"); e != nil {
		return Metadata{}, e
	}
	committed := false
	defer func() {
		if !committed {
			clean, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, e := tx.ExecContext(clean, "ROLLBACK"); e != nil {
				tx.Raw(func(any) error { return driver.ErrBadConn })
			}
		}
	}()
	var m Metadata
	var opening int64
	if e = tx.QueryRowContext(ctx, "SELECT limit_bytes,sequence,status FROM periods WHERE account=? AND period=? AND limit_kind='finite'", k.Account, k.Period).Scan(&m.Limit, &m.Sequence, &m.State); e != nil {
		return m, e
	}
	a, e := amount(ctx, tx, k)
	if e != nil {
		return m, e
	}
	m.Confirmed = a.Confirmed
	m.Pending = a.Pending
	var stamp sql.NullInt64
	if e = tx.QueryRowContext(ctx, "SELECT MIN(sampled_utc) FROM source_epochs WHERE account=? AND period=? AND incomplete=0", k.Account, k.Period).Scan(&stamp); e != nil {
		return m, e
	}
	if stamp.Valid {
		m.SampledAt = &stamp.Int64
	}
	if e = tx.QueryRowContext(ctx, "SELECT opening_bytes FROM quantities WHERE account=? AND period=?", k.Account, k.Period).Scan(&opening); e != nil {
		return m, e
	}
	if opening == 0 {
		for _, d := range []string{"up", "down"} {
			var sum sql.NullInt64
			var gaps int
			e = tx.QueryRowContext(ctx, "SELECT SUM(n),COALESCE(MAX(gap),0) FROM (SELECT actual n,0 gap FROM tickets WHERE account=? AND period=? AND direction=? AND actual IS NOT NULL UNION ALL SELECT observed-baseline,incomplete FROM source_epochs WHERE account=? AND period=? AND direction=?)", k.Account, k.Period, d, k.Account, k.Period, d).Scan(&sum, &gaps)
			if e != nil {
				return m, e
			}
			if sum.Valid && gaps == 0 {
				v := sum.Int64
				if v < 0 {
					return m, errors.New("invalid direction total")
				}
				if d == "up" {
					m.Upload = &v
				} else {
					m.Download = &v
				}
			}
		}
	}
	_, e = tx.ExecContext(ctx, "COMMIT")
	committed = e == nil
	return m, e
}
