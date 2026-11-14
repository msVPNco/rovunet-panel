package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

// The online contract permits overlapping writers only with identical ledger,
// reservation, feed identity and runtime credential semantics. Bump it for an
// incompatible schema or a migration requiring an exclusive offline cutover.
const OnlineUpdateABI = "rovunet-online-v1 ledger=1 reservations=1 feeds=1 status=2"

func hasSQLiteAccounts(cfg *config.Config) bool {
	for _, u := range cfg.Users {
		if u.SQLiteMeter != nil {
			return true
		}
	}
	return false
}

func finishInstallationAccounting(cfg *config.Config, fresh map[string]bool) error {
	if err := installAccounting(cfg, fresh); err != nil {
		return err
	}
	if e := MigrateFiniteAccounting(cfg, &actions.StdOutput{}); e != nil {
		return e
	}
	return EnsureExpiryTimer(cfg)
}

func prepareCandidateRecovery(cfg *config.Config, candidate string) (string, error) {
	if !cfg.SubscriptionV2 && !requiresCoherent(cfg) {
		return prepareUpdateRecovery(cfg)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := os.Chmod(candidate, 0755); err != nil {
		return "", err
	}
	output, err := exec.CommandContext(ctx, candidate, "update-compatibility").CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != OnlineUpdateABI {
		return "", fmt.Errorf("authenticated candidate does not declare support for this accounting and subscription format")
	}
	return coherentUpdateBackup(cfg)
}

func coherentUpdateBackup(cfg *config.Config) (string, error) {
	var err error
	root := filepath.Join(filepath.Dir(cfg.Path()), "update-backups")
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(root, "online-")
	if err != nil {
		return "", err
	}
	hash, err := CreateCoherentSnapshot(context.Background(), cfg, filepath.Join(dir, "snapshot"), config.NewID())
	if err != nil {
		return "", err
	}
	if err = safefile.Write(filepath.Join(dir, "snapshot.sha256"), []byte(hash+"\n"), 0600); err != nil {
		return "", err
	}
	return dir, safefile.SyncDir(root)
}
