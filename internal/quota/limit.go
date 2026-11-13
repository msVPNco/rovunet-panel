package quota

import (
	"context"
	"time"
)

func (s *Store) Limit(ctx context.Context, k Key) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var n int64
	e := s.db.QueryRowContext(ctx, "SELECT limit_bytes FROM periods WHERE account=? AND period=? AND status='active' AND limit_kind='finite' AND (expires_kind='none' OR (expires_kind='at' AND expires_utc>unixepoch()))", k.Account, k.Period).Scan(&n)
	return n, e
}
