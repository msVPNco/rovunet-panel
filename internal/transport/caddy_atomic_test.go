package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectedCaddyConfigKeepsWorkingSubscriptionRoute(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "Caddyfile")
	engine := filepath.Join(d, "validator")
	old := []byte("healthy subscription route and credentials")
	os.WriteFile(path, old, 0600)
	os.WriteFile(engine, []byte("#!/bin/sh\nprintf 'secret-password https://example.test/sub/private-token.json' >&2\nexit 1\n"), 0755)
	err := validateAndWriteCaddy(engine, path, []byte("new private configuration"))
	if err == nil {
		t.Fatal("rejected candidate installed")
	}
	if strings.Contains(err.Error(), "secret-password") || strings.Contains(err.Error(), "private-token") {
		t.Fatal("engine error leaked private configuration")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(old) {
		t.Fatal("valid subscription route removed by failed update")
	}
	os.WriteFile(engine, []byte("#!/bin/sh\nexit 0\n"), 0755)
	if err := validateAndWriteCaddy(engine, path, []byte("validated route")); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if string(got) != "validated route" {
		t.Fatal("validated file was not installed")
	}
	entries, _ := os.ReadDir(d)
	if len(entries) != 2 {
		t.Fatal("private validation file left behind")
	}
}
