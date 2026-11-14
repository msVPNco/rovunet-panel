package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFiniteCutoverBootGateScope(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config")
	dir := p + ".finite-upgrade"
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	marker := filepath.Join(dir, "pending.json")
	if e := os.WriteFile(marker, []byte(`{"Version":1,"Phase":"planned","Users":["alice"],"Cores":["reality"]}`), 0600); e != nil {
		t.Fatal(e)
	}
	if !FiniteUpgradePending(p, "alice") || FiniteUpgradePending(p, "bob") || !FiniteUpgradeCorePending(p, "reality") || FiniteUpgradeCorePending(p, "hy2") {
		t.Fatal("pending gate scope changed")
	}
	for _, b := range []string{`{}`, `{"Version":1,"Phase":"unknown"}`, `bad`} {
		if e := os.WriteFile(marker, []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
		if !FiniteUpgradePending(p, "alice") || !FiniteUpgradeCorePending(p, "reality") {
			t.Fatal("malformed intent failed open")
		}
	}
	if e := os.WriteFile(marker, []byte(`{"Version":1,"Phase":"ready"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if FiniteUpgradePending(p, "alice") || FiniteUpgradeCorePending(p, "reality") {
		t.Fatal("validated ready state blocked")
	}
}
