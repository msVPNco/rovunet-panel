package handlers

import (
	"errors"
	"testing"
)

func TestRandomCorePortUsesFreeHighPort(t *testing.T) {
	spec := completeCoreSpec{name: "managed-ss2022", protocol: "ss2022", ports: []int{28444, 28445}}
	called := false
	chosen, err := randomFreeCorePortWith(spec, func(port int, proto string) error {
		called = true
		if proto != "tcp,udp" { t.Fatalf("wrong transport: %s", proto) }
		if port != 23456 { t.Fatalf("unexpected port %d", port) }
		return nil
	}, func(n int) (int, error) { return 3456, nil })
	if err != nil || !called || chosen != 23456 { t.Fatalf("result %d: %v", chosen, err) }
}

func TestRandomCorePortRespectsCDNPortList(t *testing.T) {
	spec := completeCoreSpec{name: "xhttp", protocol: "xhttp", ports: []int{2096, 2087, 8880}}
	chosen, err := randomFreeCorePortWith(spec, func(port int, proto string) error {
		if proto != "tcp" { t.Fatalf("not TCP: %s", proto) }
		for _, allowed := range spec.ports { if port == allowed { return nil } }
		t.Fatalf("picked unsafe CDN port %d", port)
		return errors.New("invalid")
	}, func(n int) (int,error) { return 0,nil })
	if err != nil || chosen == 0 { t.Fatalf("CDN port missing: %v", err) }
}

func TestRandomCorePortRetriesOccupiedPortsAndFallsBack(t *testing.T) {
	spec := completeCoreSpec{name: "managed-tuic", protocol: "tuic", ports: []int{33444, 33445}}
	count := 0
	chosen, err := randomFreeCorePortWith(spec, func(port int, proto string) error {
		count++
		if port == 33444 { return nil }
		return errors.New("occupied")
	}, func(n int) (int,error) { return 0,nil })
	if err != nil || chosen != 33444 || count <= randomPortAttempts { t.Fatalf("fallback result %d attempts %d: %v", chosen,count,err) }
}

func TestRandomCorePortRejectsFailedEntropyAndExhaustion(t *testing.T) {
	spec := completeCoreSpec{name: "managed-awg", protocol: "amneziawg", ports: []int{51840}}
	if _, err := randomFreeCorePortWith(spec, func(int,string) error { return nil }, func(int)(int,error) { return 0,errors.New("rng") }); err == nil {
		t.Fatal("cryptographic RNG failure must not silently choose a port")
	}
	if _, err := randomFreeCorePortWith(spec, func(int,string) error { return errors.New("busy") }, func(int)(int,error) { return 0,nil }); err == nil {
		t.Fatal("occupied port pool should not be reported as installed")
	}
}
