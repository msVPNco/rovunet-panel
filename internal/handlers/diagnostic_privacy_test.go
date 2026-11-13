package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsDoNotCopyPrivateJournals(t *testing.T) {
	d := t.TempDir()
	script := filepath.Join(d, "journalctl")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf 'trojan://private-password@example.test/sub/private-token.json'\n"), 0755)
	t.Setenv("PATH", d+":"+os.Getenv("PATH"))
	out := &printOutput{}
	showRecentLogs(out, "rovunet-test")
	for _, line := range out.lines {
		if strings.Contains(line, "private-password") || strings.Contains(line, "private-token") {
			t.Fatal("private journal included in diagnostic report")
		}
	}
}
