package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/backup"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/usage"
	"github.com/msVPNco/rovunet-panel/internal/version"
)

func checkpointWorkspace() (string, error) {
	root := filepath.Join(config.DefaultConfigDir, "backup-staging")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("backup staging must be a regular private directory")
	}
	if err = os.Chmod(root, 0700); err != nil {
		return "", err
	}
	return os.MkdirTemp(root, "checkpoint-")
}

func encodeCoherentBackup(cfg *config.Config) (string, []string, error) {
	work, err := checkpointWorkspace()
	if err != nil {
		return "", nil, err
	}
	defer os.RemoveAll(work)
	snapshot := filepath.Join(work, "state")
	hash, err := CreateCoherentSnapshot(context.Background(), cfg, snapshot, config.NewID())
	if err != nil {
		return "", nil, err
	}
	cp := &backup.CoherentCheckpoint{ManifestSHA256: hash, Files: map[string][]byte{}}
	totalBytes := int64(0)
	err = filepath.WalkDir(snapshot, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.New("nonregular checkpoint entry")
		}
		rel, e := filepath.Rel(snapshot, p)
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		totalBytes += info.Size()
		if totalBytes > 32<<20 || len(cp.Files) >= 20000 {
			return errors.New("coherent checkpoint exceeds backup size limits")
		}
		raw, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		cp.Files[filepath.ToSlash(rel)] = raw
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	if err = cp.Validate(); err != nil {
		return "", nil, err
	}
	var frozen config.Config
	if err = json.Unmarshal(cp.Files["config.json"], &frozen); err != nil {
		return "", nil, err
	}
	b, err := backup.Create(&frozen, config.DefaultTunnelDir, version.String())
	if err != nil {
		return "", nil, err
	}
	if err = b.AddCores(coreinstall.Root); err != nil {
		return "", nil, err
	}
	b.Usage = usage.Totals()
	b.UsageHistory, err = usage.History()
	if err != nil {
		return "", nil, err
	}
	b.Format = 2
	b.Coherent = cp
	text, err := b.Encode()
	return text, b.Skipped, err
}

func restoreCoherentBackup(ctx *actions.Context, cp *backup.CoherentCheckpoint) error {
	if err := cp.Validate(); err != nil {
		return err
	}
	cfg := ctx.Config.(*config.Config)
	if !requiresCoherent(cfg) {
		return errors.New("this checkpoint requires the existing same-identity SQLite installation; use externally authenticated sealed recovery for a fresh target")
	}
	ctx.Output.Warning("Forward recovery preserves current configuration, links and credentials and only reconciles usage upward. It cannot restore a missing database or rewind a changed account.")
	if ctx.GetArg("yes") == "" {
		ok, err := prompt.Confirm("Reconcile this checkpoint with the current installation?")
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	work, err := checkpointWorkspace()
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	for name, data := range cp.Files {
		target := filepath.Join(work, filepath.FromSlash(name))
		relative, err := filepath.Rel(work, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return errors.New("invalid checkpoint target")
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err = safefile.Write(target, data, 0600); err != nil {
			return err
		}
	}
	if err = ReconcileCoherentSnapshot(context.Background(), cfg, work, cp.ManifestSHA256); err != nil {
		return fmt.Errorf("forward recovery refused without replacing current authority: %w", err)
	}
	if err = WriteSubscriptions(cfg, ctx.Output); err != nil {
		return err
	}
	ctx.Output.Success("Coherent checkpoint reconciled; current links, periods, credentials and recorded usage retained")
	return nil
}
