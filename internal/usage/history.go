package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// History contains only atomic source ledgers and the collection state. Old
// aggregate JSON totals are already carried by the backup's Usage field.
func History() (map[string][]byte, error) {
	files := map[string][]byte{}
	entries, err := os.ReadDir(Dir)
	if os.IsNotExist(err) {
		return files, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || (!strings.HasSuffix(e.Name(), ".source") && e.Name() != collectedName) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(Dir, e.Name()))
		if err != nil {
			return nil, err
		}
		files[e.Name()] = b
	}
	return files, nil
}

func ValidHistoryName(name string) bool {
	return name != "" && !strings.ContainsAny(name, `/\`) && (name == collectedName || (strings.HasPrefix(name, "xray-") && strings.HasSuffix(name, ".source")))
}

// RestoreHistory keeps any newer local ledger. A copied ledger retains its
// engine epoch, so the first sample on a different engine reports a gap.
func RestoreHistory(files map[string][]byte) error {
	if err := os.MkdirAll(Dir, 0755); err != nil {
		return err
	}
	for name, b := range files {
		if !ValidHistoryName(name) || len(b) > 1<<20 {
			return fmt.Errorf("invalid usage history file")
		}
		path := filepath.Join(Dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
