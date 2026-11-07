package coreinstall

import (
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anonvector/slipgate/internal/coreplan"
)

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"a", "reality-1", "x9"} {
		if err := ValidateName(ok); err != nil {
			t.Fatalf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-a", "A", "a/b", "../x", "a b", strings.Repeat("a", 32)} {
		if ValidateName(bad) == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func TestProto(t *testing.T) {
	want := map[string]string{"reality": "tcp", "xhttp": "tcp", "hysteria2": "udp", "wireguard": "udp"}
	for p, w := range want {
		if Proto(p) != w {
			t.Fatalf("%s: got %s want %s", p, Proto(p), w)
		}
	}
}

func TestPortFreeDetectsListener(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	if PortFree(port, "tcp") == nil {
		t.Fatal("busy tcp port reported free")
	}
	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if PortFree(c.LocalAddr().(*net.UDPAddr).Port, "udp") == nil {
		t.Fatal("busy udp port reported free")
	}
}

func TestWriteBundleRoundTripsMeta(t *testing.T) {
	Root = t.TempDir()
	o := coreplan.Options{Protocol: "hysteria2", Host: "example.org", Port: 8443, Devices: 2, Cert: "c.pem", Key: "k.pem"}
	b, err := coreplan.Generate(o)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(Root, "hy")
	if err := WriteBundle(dir, o, b); err != nil {
		t.Fatal(err)
	}
	if WriteBundle(dir, o, b) == nil {
		t.Fatal("existing core overwritten")
	}
	m, err := ReadMeta("hy")
	if err != nil || m.Protocol != "hysteria2" || m.Port != 8443 || m.Proto != "udp" {
		t.Fatalf("meta %+v %v", m, err)
	}
	list, err := List()
	if err != nil || len(list) != 1 || list[0].Name != "hy" {
		t.Fatalf("list %+v %v", list, err)
	}
}

func TestUnitRunsValidatedConfig(t *testing.T) {
	u := Unit("r1", "/etc/slipgate/core/r1")
	if u.Name != "slipgate-core-r1" || !strings.HasSuffix(u.ExecStart, "run -c /etc/slipgate/core/r1/server.json") {
		t.Fatalf("unit %+v", u)
	}
}
