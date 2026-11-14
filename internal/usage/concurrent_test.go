package usage

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestConcurrentCounterProcess(t *testing.T) {
	if dir := os.Getenv("COUNTER_CHILD_DIR"); dir != "" {
		Dir = dir
		c := NewCounter("shared")
		for i := 0; i < 100; i++ {
			c.Add("bob", 37)
			if err := c.Flush(); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	previous := Dir
	Dir = t.TempDir()
	defer func() { Dir = previous }()
	seed := NewCounter("shared")
	seed.Add("bob", 1234)
	if err := seed.Flush(); err != nil {
		t.Fatal(err)
	}
	var children []*exec.Cmd
	for i := 0; i < 4; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestConcurrentCounterProcess$")
		cmd.Env = append(os.Environ(), "COUNTER_CHILD_DIR="+Dir)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, cmd)
	}
	for _, cmd := range children {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	got, err := TotalsChecked()
	if err != nil {
		t.Fatal(err)
	}
	if got["bob"] != 1234+4*100*37 {
		t.Fatal(fmt.Sprint(got))
	}
	if err := seed.Flush(); err != nil {
		t.Fatal(err)
	}
	after, _ := TotalsChecked()
	if after["bob"] != got["bob"] {
		t.Fatal("empty retry changed totals")
	}
}
