package quota

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type writeClass struct{}
type admissionPool struct{ data, control chan struct{} }

var pools sync.Map

func pool(dir string) *admissionPool {
	p, _ := pools.LoadOrStore(dir, &admissionPool{make(chan struct{}, 16), make(chan struct{}, 4)})
	return p.(*admissionPool)
}
func schedulerFile(dir, name string) (*os.File, error) {
	path := filepath.Join(dir, name)
	f, e := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	if e = private(path, false); e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func takeSlot(ctx context.Context, ch chan struct{}) error {
	// Explicit bounded backlog: overload never opens an unreserved stream.
	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("quota admission backlog full")
	}
}
func pauseAdmission(ctx context.Context) error {
	timer := time.NewTimer(2 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// acquireWrite serializes cooperative processes before SQLite BEGIN. Control
// declares intent before waiting; new reserve/settle waits while it is pending.
// Bounds are per-process: 16 data operations, 4 control operations, one SQLite
// writer per local store. The existing three-second deadline is unchanged.
func acquireWrite(ctx context.Context, dir string, data bool) (func(), error) {
	p := pool(dir)
	slots := p.control
	if data {
		slots = p.data
	}
	if e := takeSlot(ctx, slots); e != nil {
		return nil, e
	}
	ok := false
	defer func() {
		if !ok {
			<-slots
		}
	}()
	priority, e := schedulerFile(dir, "control-intent.lock")
	if e != nil {
		return nil, e
	}
	defer func() {
		if !ok {
			priority.Close()
		}
	}()
	if !data {
		if e = syscall.Flock(int(priority.Fd()), syscall.LOCK_SH); e != nil {
			return nil, e
		}
	}
	writer, e := schedulerFile(dir, "writer-admission.lock")
	if e != nil {
		return nil, e
	}
	defer func() {
		if !ok {
			writer.Close()
		}
	}()
	probe := func() error {
		if !data {
			return nil
		}
		e := syscall.Flock(int(priority.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if e == nil {
			e = syscall.Flock(int(priority.Fd()), syscall.LOCK_UN)
		}
		return e
	}
	for {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		e = probe()
		if e == nil {
			e = syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			if e == nil {
				if e = probe(); e == nil {
					break
				}
				syscall.Flock(int(writer.Fd()), syscall.LOCK_UN)
			}
		}
		if !errors.Is(e, syscall.EWOULDBLOCK) && !errors.Is(e, syscall.EAGAIN) {
			return nil, e
		}
		if e = pauseAdmission(ctx); e != nil {
			return nil, e
		}
	}
	ok = true
	return func() { writer.Close(); priority.Close(); <-slots }, nil
}
