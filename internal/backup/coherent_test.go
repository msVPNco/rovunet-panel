package backup

import (
	"encoding/json"
	"testing"
)

func checkpointFixture(t *testing.T) *CoherentCheckpoint {
	t.Helper()
	cfg := []byte(`{"server_id":"fixture"}`)
	db := []byte("fixture sqlite bytes")
	manifest := map[string]any{"Version": 1, "ConfigHash": checkpointDigest(cfg), "Files": map[string]string{"config.json": checkpointDigest(cfg), "db-0/ledger.sqlite": checkpointDigest(db)}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return &CoherentCheckpoint{ManifestSHA256: checkpointDigest(raw), Files: map[string][]byte{"manifest.json": raw, "config.json": cfg, "db-0/ledger.sqlite": db}}
}
func TestCoherentCheckpointAuthenticatesEveryFileAndPath(t *testing.T) {
	for _, kind := range []string{"valid", "modified-db", "missing-config", "extra-file", "parent-path", "backslash-path", "absolute-path", "manifest-hash"} {
		t.Run(kind, func(t *testing.T) {
			c := checkpointFixture(t)
			switch kind {
			case "modified-db":
				c.Files["db-0/ledger.sqlite"] = []byte("changed")
			case "missing-config":
				delete(c.Files, "config.json")
			case "extra-file":
				c.Files["extra"] = nil
			case "parent-path":
				c.Files["../escape"] = nil
			case "backslash-path":
				c.Files["..\\escape"] = nil
			case "absolute-path":
				c.Files["/escape"] = nil
			case "manifest-hash":
				c.ManifestSHA256 = "wrong"
			}
			err := c.Validate()
			if kind == "valid" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unsafe checkpoint accepted")
			}
		})
	}
}
