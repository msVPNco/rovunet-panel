package quota

import (
	"context"
	"errors"
	"time"
)

// Only connection initialization and BEGIN may be retried. Never replay a
// transaction body or COMMIT: the commit outcome can be ambiguous.
func waitBusy(ctx context.Context, run func() error) error {
	for {
		err := run()
		var coded interface{ Code() int }
		if !errors.As(err, &coded) || coded.Code()&255 != 5 {
			return err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
