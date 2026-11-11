package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAbsoluteSamplesSurviveRetriesAndEngineReset(t *testing.T) {
	old := Dir
	Dir = t.TempDir()
	t.Cleanup(func() { Dir = old })
	for _, n := range []int64{100, 100, 170} {
		if err := RecordAbsolute("test-core", "epoch1", map[string]int64{"alice": n}); err != nil {
			t.Fatal(err)
		}
	}
	if got := Totals()["alice"]; got != 170 {
		t.Fatalf("retried reading double counted: %d", got)
	}
	for i := 0; i < 2; i++ {
		if err := RecordAbsolute("test-core", "epoch2", map[string]int64{"alice": 30}); err == nil {
			t.Fatal("an engine restart concealed its unsampled interval")
		}
	}
	if got := Totals()["alice"]; got != 200 {
		t.Fatalf("engine restart lost or double counted durable traffic: %d", got)
	}
	if _, err := TotalsChecked(); err == nil {
		t.Fatal("incomplete history was reported complete")
	}
}

func TestDamagedAbsoluteLedgerIsNeverReplacedWithZero(t *testing.T) {
	old := Dir
	Dir = t.TempDir()
	t.Cleanup(func() { Dir = old })
	path := filepath.Join(Dir, "test.source")
	if err := os.WriteFile(path, []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if RecordAbsolute("test", "e", map[string]int64{"alice": 10}) == nil {
		t.Fatal("damaged history accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "damaged" {
		t.Fatal("damaged ledger overwritten")
	}
}
