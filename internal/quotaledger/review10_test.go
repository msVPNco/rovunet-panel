package quotaledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReview10RestoredFloorWithPending(t *testing.T) {
	for _, actual := range []int64{0, 40, 100} {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
		store := Store{dir}
		for _, err := range []error{store.Open("a", "p", 1000, 0), store.Reserve("a", "p", "t", 100), store.RaiseFloor("a", "p", 200)} {
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := store.Settle("a", "p", "t", actual); err != nil {
			t.Fatal(err)
		}
		state, err := store.Read("a", "p")
		if err != nil {
			t.Fatal(err)
		}
		if state.Charged < 200 {
			t.Fatalf("actual=%d restored floor lost: %d", actual, state.Charged)
		}
	}
}

func TestReview10IncompleteExistingStore(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"Version":1,"Current":{},"Periods":{}} {}`, `{"Version":1,"Current":{},"Periods":{}} garbage`} {
		dir := t.TempDir()
		os.Chmod(dir, 0700)
		path := filepath.Join(dir, "ledger.json")
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		store := Store{dir}
		err := store.Open("a", "p", 1000, 0)
		after, _ := os.ReadFile(path)
		if err == nil || string(after) != raw {
			t.Errorf("accepted or rewrote incomplete store %q", raw)
		}
	}
}

func TestReview10InvalidCurrentMapping(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	store := Store{dir}
	if err := store.Open("a", "p", 1000, 0); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ledger.json")
	data, _ := os.ReadFile(path)
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	value["Current"].(map[string]any)["a"] = "missing"
	corrupt, _ := json.Marshal(value)
	os.WriteFile(path, corrupt, 0600)
	if err := store.Open("b", "p", 1000, 0); err == nil {
		t.Fatal("unrelated account write accepted inconsistent current map")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(corrupt) {
		t.Fatal("corrupted state changed")
	}
}
