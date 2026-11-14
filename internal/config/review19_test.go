package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFiniteSaveDeadlineDoesNotAcknowledge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg")
	c := Default()
	c.Users = []UserConfig{{Username: "A", Password: "p", QuotaGB: 1}}
	if e := c.SaveTo(path); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	lease, e := AdmitFinite(path, "A")
	if e != nil {
		t.Fatal(e)
	}
	defer lease.Close()
	c.RemoveUser("A")
	start := time.Now()
	if e = c.Save(); e == nil {
		t.Fatal("save acknowledged outstanding session")
	}
	if time.Since(start) < 9*time.Second {
		t.Fatal("deadline test not exercised")
	}
	after, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("failed mutation became selected")
	}
	link := filepath.Join(t.TempDir(), "alias")
	if e = os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if a, e := AdmitFinite(link, "A"); e == nil {
		a.Close()
		t.Fatal("alternate path bypass")
	}
}
