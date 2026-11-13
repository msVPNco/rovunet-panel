package proxy

import (
	"context"
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/keys"
	"github.com/msVPNco/rovunet-panel/internal/quota"
	"io"
	"log"
	"math"
	"net"
	"os"
	"time"
)

// previewRelay is opt-in for a finite-quota native TCP canary.
func (s *VLESSServer) quotaBinding(ctx context.Context, id [16]byte, user string) (quota.Key, int64, error) {
	cfg, e := config.LoadFrom(s.opts.UsersFile)
	if e != nil {
		return quota.Key{}, 0, e
	}
	if config.AccountOnHold(s.opts.UsersFile, user) || cfg.RecoverySealed {
		return quota.Key{}, 0, quota.ErrConflict
	}
	u := cfg.GetUser(user)
	if u == nil || u.Expired(time.Now()) || u.QuotaGB <= 0 || u.QuotaGB > math.MaxInt64/(1<<30) || keys.VLESSUserID(s.secret, u.Username, u.Password) != id || cfg.ServerID != s.opts.QuotaServer {
		return quota.Key{}, 0, quota.ErrConflict
	}
	if u.SQLiteMeter != nil && !s.opts.QuotaStore.MatchesBinding(u.SQLiteMeter.Directory, cfg.ServerID, u.SQLiteMeter.Store) {
		return quota.Key{}, 0, quota.ErrConflict
	}
	k := quota.Key{Account: u.AccountID, Period: u.PeriodID}
	limit, e := s.opts.QuotaStore.Limit(ctx, k)
	if e != nil || limit != u.QuotaBytes() {
		return k, 0, quota.ErrConflict
	}
	a, e := s.opts.QuotaStore.Read(ctx, k)
	if e != nil {
		return k, 0, e
	}
	if a.Confirmed >= limit {
		return k, 0, quota.ErrBudget
	}
	return k, limit - a.Confirmed - a.Pending, nil
}
func (s *VLESSServer) quotaTCP(ctx context.Context, req *vlessRequest, k quota.Key, lease *config.AccessLease, ws *wsConn, client io.Reader, up net.Conn) {
	closeUp := context.AfterFunc(ctx, func() { _ = up.Close() })
	defer closeUp()
	done := make(chan struct{}, 2)
	fault := func(point, dir string) error {
		if s.opts.quotaFault != nil {
			return s.opts.quotaFault(point, req.user, dir)
		}
		return nil
	}
	pump := func(r io.Reader, dir string, write func([]byte) (int, error)) {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 32<<10)
		for {
			n, re := r.Read(buf)
			for off := 0; off < n; {
				bind, free, e := s.quotaBinding(ctx, req.id, req.user)
				if e != nil || bind != k || free <= 0 {
					quotaDiagnostic("binding", e)
					return
				}
				count := min(int64(n-off), free)
				op := config.NewID()
				if e = s.opts.QuotaStore.ReserveNative(ctx, k, op, dir, count); e != nil {
					if errors.Is(e, quota.ErrBudget) {
						continue
					}
					quotaDiagnostic("reserve", e)
					return
				}
				// A failed/lost reserve acknowledgment never permits a write. The
				// durable grant stays pending until a trusted reconciliation.
				if fault("reserve-reply", dir) != nil {
					return
				}
				written := 0
				we := ctx.Err()
				if we == nil {
					we = lease.Check()
				}
				if we == nil {
					written, we = write(buf[off : off+int(count)])
				}

				if written < 0 {
					return
				} // framed short write with uncertain payload extent
				if written > int(count) {
					return
				}
				finish, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				e = fault("before-settle", dir)
				if e == nil {
					e = s.opts.QuotaStore.Settle(finish, k, op, int64(written))
				}
				if e == nil {
					e = fault("settle-reply", dir)
				}
				cancel()
				if e != nil || we != nil || written == 0 {
					quotaDiagnostic("settle", e)
					quotaDiagnostic("socket-write", we)
					return
				}
				off += written
				if _, _, e = s.quotaBinding(ctx, req.id, req.user); e != nil {
					return
				}
			}
			if re != nil {
				return
			}
		}
	}
	go pump(client, "up", up.Write)
	go pump(up, "down", func(b []byte) (int, error) {
		if e := ws.writeMessage(b); e != nil {
			return -1, e
		}
		return len(b), nil
	})
	<-done
	_ = up.Close()
	_ = ws.conn.Close()
	<-done
}

// Opt-in operational diagnostics contain no user, credential or endpoint.
func quotaDiagnostic(stage string, err error) {
	if err == nil || os.Getenv("ROVUNET_QUOTA_DIAGNOSTICS") != "1" {
		return
	}
	kind := "other"
	if errors.Is(err, context.DeadlineExceeded) {
		kind = "deadline"
	} else if errors.Is(err, context.Canceled) {
		kind = "canceled"
	} else if errors.Is(err, quota.ErrBudget) {
		kind = "budget"
	} else if errors.Is(err, quota.ErrConflict) {
		kind = "identity-conflict"
	}
	var coded interface{ Code() int }
	if errors.As(err, &coded) {
		log.Printf("finite quota failure stage=%s sqlite_code=%d", stage, coded.Code())
		return
	}
	log.Printf("finite quota failure stage=%s class=%s", stage, kind)
}
