package usage

import (
	"errors"
	"os"
	"testing"
)

func TestVisibleCommitRetryDoesNotDoubleCount(t *testing.T) {
	oldDir := Dir
	Dir = t.TempDir()
	defer func() { Dir = oldDir }()
	oldWriter := writeCounter
	defer func() { writeCounter = oldWriter }()
	failed := false
	writeCounter = func(path string, data []byte, mode os.FileMode) error {
		if err := oldWriter(path, data, mode); err != nil {
			return err
		}
		if !failed {
			failed = true
			return errors.New("injected directory sync acknowledgment failure")
		}
		return nil
	}
	counter := NewCounter("retry")
	counter.Add("bob", 1234)
	if err := counter.Flush(); err == nil {
		t.Fatal("missing injected failure")
	}
	if err := counter.Flush(); err != nil {
		t.Fatal(err)
	}
	counter.Add("bob", 10)
	if err := counter.Flush(); err != nil {
		t.Fatal(err)
	}
	totals, err := TotalsChecked()
	if err != nil || totals["bob"] != 1244 {
		t.Fatalf("retry duplicated bytes: %v %v", totals, err)
	}
}
