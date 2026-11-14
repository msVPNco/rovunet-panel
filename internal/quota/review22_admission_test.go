package quota

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestControlAdmissionPriority22(t *testing.T) {
	ctx, _, dir, _ := newTestStore(t)
	hold, e := acquireWrite(ctx, dir, true)
	must(t, e)
	control := make(chan func(), 1)
	failed := make(chan error, 2)
	limited, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	go func() {
		release, e := acquireWrite(limited, dir, false)
		if e != nil {
			failed <- e
		} else {
			control <- release
		}
	}()
	signal, e := schedulerFile(dir, "control-intent.lock")
	must(t, e)
	defer signal.Close()
	pending := false
	for i := 0; i < 100; i++ {
		e = syscall.Flock(int(signal.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if e == syscall.EWOULDBLOCK {
			pending = true
			break
		}
		syscall.Flock(int(signal.Fd()), syscall.LOCK_UN)
		time.Sleep(time.Millisecond)
	}
	if !pending {
		t.Fatal("control intent not visible")
	}
	data := make(chan func(), 1)
	go func() {
		release, e := acquireWrite(limited, dir, true)
		if e != nil {
			failed <- e
		} else {
			data <- release
		}
	}()
	hold()
	select {
	case release := <-control:
		select {
		case releaseData := <-data:
			releaseData()
			t.Fatal("data bypassed pending control")
		default:
		}
		release()
	case e := <-failed:
		t.Fatal(e)
	case <-time.After(time.Second):
		t.Fatal("control starved")
	}
	select {
	case release := <-data:
		release()
	case e := <-failed:
		t.Fatal(e)
	case <-time.After(time.Second):
		t.Fatal("data did not resume")
	}
	// No database reset is involved in replacing ephemeral scheduling state.
	if _, e = os.Stat(dir + "/ledger.sqlite"); e != nil {
		t.Fatal(e)
	}
}
