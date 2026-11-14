package coreinstall

import (
	"debug/buildinfo"
	"os"
	"path/filepath"
	"testing"
)

func TestReviewedRuntimeRejectsMetadataPreservingAppend(t *testing.T) {
	path := os.Getenv("ROVUNET_REVIEWED_XRAY")
	if path == "" {
		t.Skip("requires the exact reviewed runtime fixture")
	}
	if err := verifyGoSource(path, XrayCommit); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := filepath.Join(t.TempDir(), "xray")
	if err = os.WriteFile(changed, append(bytes, []byte("unreviewed appended payload")...), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = buildinfo.ReadFile(changed); err != nil {
		t.Fatal("fixture no longer tests intact Go metadata", err)
	}
	if err = verifyGoSource(changed, XrayCommit); err == nil {
		t.Fatal("accepted changed executable with original source metadata")
	}
}
