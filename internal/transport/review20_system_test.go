package transport

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"os"
	"testing"
)

func TestReview20CreateCaddy(t *testing.T) {
	if os.Getenv("RV20_INSTALLED_VM") != "owned-review20" {
		t.Skip("owned full VM only")
	}
	if _, e := os.Stat("/etc/rovunet-lab-authorized"); e != nil {
		t.Fatal(e)
	}
	cfg, e := config.Load()
	if e != nil {
		t.Fatal(e)
	}
	tun := cfg.GetTunnel("review20-naive")
	if tun == nil || tun.Domain != "127.0.0.1" {
		t.Fatal("unexpected fixture")
	}
	if e = os.MkdirAll(config.TunnelDir(tun.Tag), 0700); e != nil {
		t.Fatal(e)
	}
	if e = CreateService(tun, cfg); e != nil {
		t.Fatal(e)
	}
}
func TestLockedFallbackStableOnRefresh(t *testing.T) {
	a := "  basic_auth locked-123 456"
	b := "  basic_auth locked-789 abc"
	if preserveLockedAuth(a, b) != a {
		t.Fatal("no-access credential changed on identical eligible-user set")
	}
	if preserveLockedAuth(a, "  basic_auth actual-user password") != "  basic_auth actual-user password" {
		t.Fatal("retained obsolete authentication")
	}
}
