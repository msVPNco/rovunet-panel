package coreinstall

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

func sameJSON(a, b []byte) bool {
	var x, y bytes.Buffer
	return json.Compact(&x, a) == nil && json.Compact(&y, b) == nil && bytes.Equal(x.Bytes(), y.Bytes())
}

const appliedFile = "applied.json"

// Applied is the last successfully applied server config and its client
// files. Desired files may change during a retry; exports use this snapshot
// to avoid presenting an addition or password change the engine rejected.
type applied struct {
	Server  json.RawMessage   `json:"server"`
	Clients map[string]string `json:"clients"`
}

func snapshotApplied(dir string) (applied, error) {
	server, err := os.ReadFile(filepath.Join(dir, "server.json"))
	if err != nil {
		return applied{}, err
	}
	a := applied{Server: server, Clients: map[string]string{}}
	entries, err := os.ReadDir(filepath.Join(dir, "clients"))
	if err != nil {
		return applied{}, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "clients", e.Name()))
		if err != nil {
			return applied{}, err
		}
		a.Clients[e.Name()] = string(b)
	}
	return a, nil
}

func saveApplied(dir string) error {
	a, err := snapshotApplied(dir)
	if err != nil {
		return err
	}
	return commitApplied(dir, a)
}

func commitApplied(dir string, a applied) error {
	path := filepath.Join(dir, appliedFile)
	if err := writeJSON(path+".tmp", a); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func loadApplied(dir string) (applied, error) {
	var a applied
	err := readJSON(filepath.Join(dir, appliedFile), &a)
	return a, err
}
