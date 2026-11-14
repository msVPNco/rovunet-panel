package handlers

import (
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/version"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Entry point for the already authenticated executable staged by install.sh.
// Every mutation uses the same lock, recovery and permanent-path post-update as menu 8.
func UpgradeInstalled(target string) error {
	if os.Geteuid() != 0 || target != "/usr/local/bin/rovunet" {
		return errors.New("invalid upgrade destination")
	}
	src, err := os.Executable()
	if err != nil {
		return err
	}
	if src == target {
		return errors.New("bootstrap upgrade requires a staged release")
	}
	lock, err := os.OpenFile(filepath.Join(filepath.Dir(target), ".rovunet-update.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	tx, resume, err := binary.ResumeUpdate(target)
	if err != nil {
		return err
	}
	if resume {
		if err = runStrictPostUpdate(target, tx); err != nil {
			return err
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.RecoverySealed {
		return config.ErrCoordinatedRecovery
	}
	previous, err := exec.Command(target, "--version").CombinedOutput()
	if err != nil {
		return err
	}
	words := strings.Fields(string(previous))
	if len(words) < 2 || words[0] != "rovunet" {
		return errors.New("unrecognized installed version")
	}
	if err = binary.CheckUpgradeOrder(words[1], version.Version); err != nil {
		return err
	}
	oldHash, err := binary.FileSHA256(target)
	if err != nil {
		return err
	}
	newHash, err := binary.FileSHA256(src)
	if err != nil {
		return err
	}
	if oldHash == newHash {
		if _, e := os.Stat(filepath.Join(filepath.Dir(target), ".rovunet-install-state.json")); e == nil {
			return errors.New("verified bootstrap pending: run rovunet install to finish setup")
		} else if !os.IsNotExist(e) {
			return e
		}
		return nil
	}
	recovery, err := prepareCandidateRecovery(cfg, src)
	if err != nil {
		return err
	}
	tx, err = binary.BeginUpdate(target, src, recovery)
	if err != nil {
		return err
	}
	return runStrictPostUpdate(target, tx)
}
