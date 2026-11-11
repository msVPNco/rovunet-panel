package coreinstall

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectedNewConfigRollsBackTheCreatedDirectory(t *testing.T) {
	oldRoot, oldXray := Root, XrayPath
	Root = t.TempDir()
	XrayPath = filepath.Join(t.TempDir(), "reject-runtime")
	t.Cleanup(func() { Root, XrayPath = oldRoot, oldXray })
	if err := os.WriteFile(XrayPath, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	u, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := u.LocalAddr().(*net.UDPAddr).Port
	_ = u.Close()
	o := hy2
	o.Port = port
	dir, err := InstallForUsers("reject-unit-test", o, []config.UserConfig{{Username: "alice", Password: "fixture"}})
	if err == nil || dir != "" {
		t.Fatal("rejected installation was reported installed")
	}
	if _, err := os.Stat(filepath.Join(Root, "reject-unit-test")); !os.IsNotExist(err) {
		t.Fatal("failed installation left its private configuration behind")
	}
}
