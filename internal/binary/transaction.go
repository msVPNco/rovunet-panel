package binary

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

type UpdateTransaction struct {
	Schema          int    `json:"schema"`
	Target          string `json:"target"`
	Backup          string `json:"backup"`
	Recovery        string `json:"recovery"`
	PreviousSHA256  string `json:"previous_sha256"`
	InstalledSHA256 string `json:"installed_sha256"`
	Phase           string `json:"phase"`
}

func updateStatePath(target string) string {
	return filepath.Join(filepath.Dir(target), ".rovunet-update-state.json")
}

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ReadUpdateTransaction(target string) (*UpdateTransaction, error) {
	f, err := os.Open(updateStatePath(target))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil {
		return nil, err
	}
	if len(b) > 65536 {
		return nil, errors.New("update recovery record too large")
	}
	var tx UpdateTransaction
	if json.Unmarshal(b, &tx) != nil || tx.Schema != 1 || tx.Target != target || tx.Backup != target+".bak" || len(tx.PreviousSHA256) != 64 || len(tx.InstalledSHA256) != 64 {
		return nil, errors.New("invalid update recovery record; preserve it for inspection")
	}
	switch tx.Phase {
	case "prepared", "installed", "post-update-pending", "complete", "aborted-before-install":
	default:
		return nil, errors.New("unknown update recovery phase")
	}
	return &tx, nil
}

func (tx *UpdateTransaction) SavePhase(phase string) error {
	tx.Phase = phase
	b, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return err
	}
	return safefile.Write(updateStatePath(tx.Target), append(b, '\n'), 0600)
}

// BeginUpdate never unlinks/truncates a working binary. A failed rename leaves
// it present, including EXDEV/no-space/permission errors. The recovery binary
// and configuration snapshot survive post-update and are not deleted early.
func BeginUpdate(target, candidate, recovery string) (*UpdateTransaction, error) {
	if info, err := os.Lstat(target); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("update target must be an existing regular file")
	}
	old, err := FileSHA256(target)
	if err != nil {
		return nil, err
	}
	newHash, err := FileSHA256(candidate)
	if err != nil {
		return nil, err
	}
	previous, err := ReadUpdateTransaction(target)
	if err != nil {
		return nil, err
	}
	if previous != nil && previous.Phase != "complete" && previous.Phase != "aborted-before-install" {
		return nil, errors.New("an earlier update is pending; resume it before replacing its recovery binary")
	}
	tx := &UpdateTransaction{Schema: 1, Target: target, Backup: target + ".bak", Recovery: recovery, PreviousSHA256: old, InstalledSHA256: newHash}
	if err := safefile.Copy(target, tx.Backup, 0755); err != nil {
		return nil, fmt.Errorf("retain previous executable: %w", err)
	}
	if err := tx.SavePhase("prepared"); err != nil {
		return nil, err
	}
	if err := os.Rename(candidate, target); err != nil {
		return nil, fmt.Errorf("atomic replacement failed; previous binary remains installed: %w", err)
	}
	if err := safefile.SyncDir(filepath.Dir(target)); err != nil {
		return tx, err
	}
	if err := tx.SavePhase("installed"); err != nil {
		return tx, err
	}
	return tx, nil
}

// ResumeUpdate detects both sides of a crash at Rename. A previous panel
// still installed can safely start a fresh verified attempt; a new panel
// resumes its own post-update. An unrelated executable is never overwritten.
func ResumeUpdate(target string) (*UpdateTransaction, bool, error) {
	tx, err := ReadUpdateTransaction(target)
	if err != nil || tx == nil {
		return tx, false, err
	}
	if tx.Phase == "complete" || tx.Phase == "aborted-before-install" {
		return tx, false, nil
	}
	current, err := FileSHA256(target)
	if err != nil {
		return tx, false, err
	}
	if current == tx.InstalledSHA256 {
		return tx, true, nil
	}
	if current == tx.PreviousSHA256 && tx.Phase == "prepared" {
		return tx, false, tx.SavePhase("aborted-before-install")
	}
	return tx, false, errors.New("installed executable differs from the pending update record; preserve the recovery files for inspection")
}
