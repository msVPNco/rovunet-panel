package quotaledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReview10StrictStateMatrix(t *testing.T) {
	original := Store{privateDir(t)}
	if err := original.Open("a", "p", 1000, 10); err != nil {
		t.Fatal(err)
	}
	if err := original.Reserve("a", "p", "ticket", 100); err != nil {
		t.Fatal(err)
	}
	if err := original.Observe("a", "p", "source:epoch", 20); err != nil {
		t.Fatal(err)
	}
	validBytes, err := os.ReadFile(filepath.Join(original.Directory, "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{"empty-object": []byte(`{}`), "null": []byte(`null`), "second-value": append(append([]byte{}, validBytes...), []byte(` {}`)...), "trailing-junk": append(append([]byte{}, validBytes...), []byte(` garbage`)...)}
	makeCase := func(name string, change func(map[string]any)) {
		var value map[string]any
		json.Unmarshal(validBytes, &value)
		change(value)
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		cases[name] = b
	}
	for _, name := range []string{"Version", "Current", "Periods"} {
		name := name
		makeCase("missing-root-"+name, func(v map[string]any) { delete(v, name) })
	}
	period := func(v map[string]any) map[string]any {
		return v["Periods"].(map[string]any)[periodKey("a", "p")].(map[string]any)
	}
	for _, name := range []string{"Limit", "Charged", "Confirmed", "Floor", "Closed", "Tickets", "Sources"} {
		name := name
		makeCase("missing-period-"+name, func(v map[string]any) { delete(period(v), name) })
	}
	makeCase("current-missing", func(v map[string]any) { v["Current"].(map[string]any)["a"] = "absent" })
	makeCase("current-closed", func(v map[string]any) { period(v)["Closed"] = true })
	makeCase("orphan-period", func(v map[string]any) { v["Current"] = map[string]any{} })
	makeCase("ticket-negative", func(v map[string]any) {
		period(v)["Tickets"].(map[string]any)["ticket"].(map[string]any)["Reserved"] = -1
	})
	makeCase("ticket-actual-over", func(v map[string]any) {
		period(v)["Tickets"].(map[string]any)["ticket"].(map[string]any)["Actual"] = 101
	})
	makeCase("ticket-actual-negative", func(v map[string]any) {
		period(v)["Tickets"].(map[string]any)["ticket"].(map[string]any)["Actual"] = -1
	})
	makeCase("ticket-missing-actual", func(v map[string]any) {
		delete(period(v)["Tickets"].(map[string]any)["ticket"].(map[string]any), "Actual")
	})
	makeCase("source-negative", func(v map[string]any) { period(v)["Sources"].(map[string]any)["source:epoch"] = -1 })
	makeCase("source-whitespace", func(v map[string]any) { period(v)["Sources"].(map[string]any)[" "] = 1 })
	makeCase("source-null", func(v map[string]any) { period(v)["Sources"] = nil })
	makeCase("source-value-null", func(v map[string]any) { period(v)["Sources"].(map[string]any)["source:epoch"] = nil })
	makeCase("wrong-charge", func(v map[string]any) { period(v)["Charged"] = 0 })
	makeCase("floor-over-confirmed", func(v map[string]any) { period(v)["Floor"] = 999 })
	makeCase("null-closed", func(v map[string]any) { period(v)["Closed"] = nil })
	cases["duplicate-member"] = []byte(`{"Version":2,"Version":2,"Current":{},"Periods":{}}`)
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			dir := privateDir(t)
			path := filepath.Join(dir, "ledger.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := (Store{dir}).Open("b", "p", 1000, 0); err == nil {
				t.Fatal("accepted invalid existing state")
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(raw) {
				t.Fatal("invalid state bytes changed")
			}
		})
	}
}

func TestReview10MissingInitializedStore(t *testing.T) {
	store := Store{privateDir(t)}
	if err := store.Open("a", "p", 1000, 123); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Directory, "ledger.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := store.Open("a", "p", 1000, 0); err == nil {
		t.Fatal("deleted initialized state reset to zero")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("missing store was rewritten")
	}
}
