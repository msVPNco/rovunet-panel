package quotaledger

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestReview10ConfirmedFloorModel(t *testing.T) {
	for _, actual := range []int64{0, 40, 100} {
		t.Run(fmt.Sprint(actual), func(t *testing.T) {
			store := Store{privateDir(t)}
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(store.Open("a", "p", 1000, 0))
			must(store.Reserve("a", "p", "ticket", 100))
			must(store.RaiseFloor("a", "p", 200))
			before, err := store.Read("a", "p")
			must(err)
			if before.Floor != 200 || before.Confirmed != 200 || before.Charged != 300 {
				t.Fatal(before)
			}
			reopened := Store{store.Directory}
			must(reopened.Settle("a", "p", "ticket", actual))
			must(reopened.Settle("a", "p", "ticket", actual))
			must(reopened.RaiseFloor("a", "p", 200))
			must(reopened.RaiseFloor("a", "p", 200))
			after, err := reopened.Read("a", "p")
			must(err)
			if after.Confirmed != 200+actual || after.Charged != 200+actual || after.Floor != 200 {
				t.Fatal(after)
			}
			must(reopened.Reserve("a", "p", "next", 50))
			must(reopened.Settle("a", "p", "next", 50))
			after, err = reopened.Read("a", "p")
			must(err)
			if after.Charged != 250+actual {
				t.Fatal("future transfer was absorbed into restore floor", after)
			}
			must(reopened.Renew("a", "p", "p2", 1000))
			if err := store.RaiseFloor("a", "p", 300); err == nil {
				t.Fatal("stale period accepted a restore")
			}
			t.Logf("actual=%d, floor=200, confirmed=%d, charged=%d after future50", actual, after.Confirmed, after.Charged)
		})
	}
}

func TestReview10RestoreSettleProcesses(t *testing.T) {
	if dir := os.Getenv("REVIEW10_FLOOR_CHILD"); dir != "" {
		store := Store{dir}
		var err error
		if os.Getenv("REVIEW10_ACTION") == "floor" {
			err = store.RaiseFloor("a", "p", 200)
		} else {
			err = store.Settle("a", "p", "ticket", 0)
		}
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	for attempt := 0; attempt < 12; attempt++ {
		store := Store{privateDir(t)}
		if err := store.Open("a", "p", 1000, 0); err != nil {
			t.Fatal(err)
		}
		if err := store.Reserve("a", "p", "ticket", 100); err != nil {
			t.Fatal(err)
		}
		var children []*exec.Cmd
		for _, action := range []string{"floor", "settle"} {
			command := exec.Command(os.Args[0], "-test.run=^TestReview10RestoreSettleProcesses$")
			command.Env = append(os.Environ(), "REVIEW10_FLOOR_CHILD="+store.Directory, "REVIEW10_ACTION="+action)
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			children = append(children, command)
		}
		for _, command := range children {
			if err := command.Wait(); err != nil {
				t.Fatal(err)
			}
		}
		state, err := store.Read("a", "p")
		if err != nil || state.Charged != 200 || state.Confirmed != 200 || state.Floor != 200 {
			t.Fatalf("%v %+v", err, state)
		}
	}
}
