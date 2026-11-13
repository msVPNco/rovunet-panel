package handlers

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"os"
	"testing"
	"time"
)

func TestReview19SystemRotationBarrier(t *testing.T) {
	if os.Getenv("RV19_INSTALLED_VM") != "owned-review19" {
		t.Skip("requires disposable owned VM")
	}
	if _, e := os.Stat("/etc/rovunet-lab-authorized"); e != nil {
		t.Fatal("missing owned lab marker")
	}
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	if subscriptionHost(cfg) != "127.0.0.1:19446" {
		t.Fatal("unexpected target")
	}
	point := os.Getenv("RV19_POWER_PHASE")
	publicationBarrier = func(p string) error {
		if p == point {
			if e := os.WriteFile("/root/review19/barrier", []byte(p), 0600); e != nil {
				return e
			}
			for {
				time.Sleep(100 * time.Millisecond)
			}
		}
		return nil
	}
	if e = RotateSubscriptionOperation(cfg, "alice", os.Getenv("RV19_OPERATION")); e != nil {
		t.Fatal(e)
	}
}
