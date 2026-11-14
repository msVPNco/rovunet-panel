package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
)

// CoherentCheckpoint is an online forward-recovery checkpoint. It is not
// authority to replace a missing/live database or rewind config, tokens or usage.
type CoherentCheckpoint struct {
	ManifestSHA256 string            `json:"manifest_sha256"`
	Files          map[string][]byte `json:"files"`
}

func checkpointDigest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func (c *CoherentCheckpoint) Validate() error {
	if c == nil || len(c.Files) == 0 || len(c.Files) > 20000 {
		return errors.New("invalid coherent checkpoint file count")
	}
	total := 0
	for name, data := range c.Files {
		if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.Contains(name, "\\") || name == ".." || strings.HasPrefix(name, "../") {
			return errors.New("invalid coherent checkpoint path")
		}
		total += len(data)
		if total > maxDecoded {
			return errors.New("coherent checkpoint is too large")
		}
	}
	raw, ok := c.Files["manifest.json"]
	if !ok || checkpointDigest(raw) != c.ManifestSHA256 {
		return errors.New("coherent checkpoint manifest hash mismatch")
	}
	var m struct {
		Version    int
		ConfigHash string
		Files      map[string]string
	}
	if json.Unmarshal(raw, &m) != nil || m.Version != 1 || len(m.Files) != len(c.Files)-1 {
		return errors.New("invalid coherent checkpoint manifest")
	}
	for name, want := range m.Files {
		data, ok := c.Files[name]
		if !ok || name == "manifest.json" || checkpointDigest(data) != want {
			return errors.New("coherent checkpoint content hash mismatch")
		}
	}
	config, ok := c.Files["config.json"]
	if !ok || checkpointDigest(config) != m.ConfigHash {
		return errors.New("coherent checkpoint config hash mismatch")
	}
	return nil
}
