package transport

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"os"
	"strings"
	"testing"
)

func TestReview19InstalledUnit(t *testing.T) {
	if os.Getenv("RV19_INSTALLED_VM") != "owned-review19" {
		t.Skip("requires disposable owned VM")
	}
	if _, e := os.Stat("/etc/rovunet-lab-authorized"); e != nil {
		t.Fatal("missing lab marker")
	}
	c, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	tun := c.GetTunnel("review19-naive")
	if tun == nil || tun.Domain != "127.0.0.1" {
		t.Fatal("unexpected lab config")
	}
	if e = os.MkdirAll(config.TunnelDir(tun.Tag), 0700); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("RV19_REGENERATE") == "1" {
		e = RegenerateService(tun, c)
	} else {
		e = CreateService(tun, c)
	}
	if e != nil {
		t.Fatal(e)
	}
	unit, e := os.ReadFile("/etc/systemd/system/rovunet-review19-naive.service")
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"ExecStartPre=+/usr/local/bin/rovunet subscriptions-before-serve", "User=root", "WorkingDirectory=/etc/rovunet/tunnels/review19-naive"} {
		if !strings.Contains(string(unit), s) {
			t.Fatal("unit missing", s)
		}
	}
	t.Log("Actual panel service generation/apply completed with startup recovery")
}
