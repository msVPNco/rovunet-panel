package config

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStaleCommandCannotUndoTokenOrAccountChange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Users = []UserConfig{{Username: "alice", Password: "pw", SubToken: NewSubToken()}}
	if err := cfg.SaveTo(p); err != nil {
		t.Fatal(err)
	}
	older, _ := LoadFrom(p)
	current, _ := LoadFrom(p)
	current.Users[0].SubToken = NewSubToken()
	if err := current.Save(); err != nil {
		t.Fatal(err)
	}
	older.PublicAddress = "192.0.2.8"
	if err := older.Save(); !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("stale overwrite accepted: %v", err)
	}
	if err := older.RefreshIfChanged(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(older.Users, current.Users) {
		t.Fatal("account/token change was lost")
	}
	current.Users = nil
	current.ExtraAddresses = nil
	current.ServerID = ""
	if err := current.Save(); err != nil {
		t.Fatal(err)
	}
	if err := older.Reload(); err != nil {
		t.Fatal(err)
	}
	if len(older.Users) != 0 || older.ServerID != "" {
		t.Fatal("reload resurrected removed optional fields")
	}
}
