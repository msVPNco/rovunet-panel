package network

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrackedPortsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ports")
	if ports, err := readTrackedPorts(path); err != nil || ports != nil {
		t.Fatalf("missing file: %v %v", ports, err)
	}
	// Junk, duplicates and out-of-range lines are ignored.
	data := "tcp 443\nudp 53\ntcp 443\nicmp 1\ntcp 99999\nudp x\n\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	ports, err := readTrackedPorts(path)
	if err != nil || len(ports) != 2 || ports[0] != (trackedPort{"tcp", 443}) || ports[1] != (trackedPort{"udp", 53}) {
		t.Fatalf("got %v %v", ports, err)
	}
	ports = updateTrackedPorts(ports, trackedPort{"tcp", 443}, true)
	if len(ports) != 2 {
		t.Fatalf("re-adding duplicated a port: %v", ports)
	}
	ports = updateTrackedPorts(ports, trackedPort{"udp", 53}, false)
	if len(ports) != 1 || ports[0].Port != 443 {
		t.Fatalf("remove: %v", ports)
	}
}

func TestBootRulesOnlyInsertRovunetPorts(t *testing.T) {
	rules := renderBootRules([]trackedPort{{"tcp", 443}, {"udp", 53}})
	for _, want := range []string{"*filter\n", "-I INPUT -p tcp --dport 443 -j ACCEPT\n", "-I INPUT -p udp --dport 53 -j ACCEPT\n", "COMMIT\n"} {
		if !strings.Contains(rules, want) {
			t.Fatalf("missing %q in:\n%s", want, rules)
		}
	}
	// No policies, chain definitions or other tables that would override
	// rules other tools load (Docker NAT, fail2ban).
	for _, bad := range []string{"*nat", ":INPUT", "-A ", "DOCKER"} {
		if strings.Contains(rules, bad) {
			t.Fatalf("unexpected %q in:\n%s", bad, rules)
		}
	}
	if !strings.Contains(iptablesUnitTemplate, "--noflush") {
		t.Fatal("boot restore must not flush other rules")
	}
}
