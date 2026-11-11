package coreinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

func TestDedicatedRuntimeUsesSeparateBinaryAndKeepsLegacyUnit(t *testing.T) {
	oldRoot, oldXray := Root, XrayPath
	t.Cleanup(func() { Root, XrayPath = oldRoot, oldXray })
	Root = t.TempDir()
	XrayPath = filepath.Join(t.TempDir(), "xray")
	marker := filepath.Join(t.TempDir(), "called")
	for path, name := range map[string]string{XrayPath: "legacy", dedicatedXrayPath(): "dedicated"} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '"+name+"' > '"+marker+"'\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dedicated := range []bool{false, true} {
		dir := filepath.Join(Root, "fixture")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := writeJSON(filepath.Join(dir, optionsFile), coreplan.Options{Protocol: "trojan", DedicatedXray: dedicated}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "meta"), []byte("protocol=trojan\nport=30444\nproto=tcp\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := TestConfig(filepath.Join(dir, "server.json")); err != nil {
			t.Fatal(err)
		}
		called, _ := os.ReadFile(marker)
		want, path := "legacy", XrayPath
		if dedicated {
			want, path = "dedicated", dedicatedXrayPath()
		}
		if string(called) != want || !strings.HasPrefix(Unit("fixture", dir).ExecStart, path+" run") {
			t.Fatal("runtime selection did not survive stored options")
		}
	}
}

func TestDedicatedRuntimeCopiesVerifiedSourceWithoutReplacingLegacy(t *testing.T) {
	engine := os.Getenv("XRAY")
	if engine == "" {
		t.Skip("set XRAY to the pinned engine")
	}
	oldXray := XrayPath
	t.Cleanup(func() { XrayPath = oldXray })
	XrayPath = filepath.Join(t.TempDir(), "xray")
	data, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(XrayPath, data, 0755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(XrayPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureOptionsEngine(coreplan.Options{Protocol: "trojan", DedicatedXray: true}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(XrayPath)
	if !os.SameFile(before, after) {
		t.Fatal("legacy runtime was replaced")
	}
	legacy, _ := os.ReadFile(XrayPath)
	copy, _ := os.ReadFile(dedicatedXrayPath())
	if string(legacy) != string(data) || string(copy) != string(data) {
		t.Fatal("runtime bytes differ")
	}
	if err := verifyGoSource(dedicatedXrayPath(), XrayCommit); err != nil {
		t.Fatal(err)
	}
}

func TestDedicatedRuntimeRejectsUnverifiedExistingFile(t *testing.T) {
	oldXray := XrayPath
	t.Cleanup(func() { XrayPath = oldXray })
	XrayPath = filepath.Join(t.TempDir(), "xray")
	bad := []byte("unverified runtime")
	if err := os.WriteFile(dedicatedXrayPath(), bad, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ensureOptionsEngine(coreplan.Options{Protocol: "ss2022", DedicatedXray: true}); err == nil {
		t.Fatal("unverified runtime accepted")
	}
	got, _ := os.ReadFile(dedicatedXrayPath())
	if string(got) != string(bad) {
		t.Fatal("unverified existing binary overwritten")
	}
}
