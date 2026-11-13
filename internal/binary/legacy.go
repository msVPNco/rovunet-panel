package binary

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

// InstallReviewedTransportAt is used by the retained standalone Naive setup.
// It never trusts an arbitrary existing binary or runs it to identify it. The
// caller owns the fixed destination and all subsequent service/config changes.
func InstallReviewedTransportAt(name, dest string) error {
	a, _, err := transportAsset(name, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	exists := false
	if st, err := os.Lstat(dest); err == nil {
		if !st.Mode().IsRegular() {
			return errors.New("runtime destination is not a regular file")
		}
		exists = true
		if verifyFile(dest, a.Size, a.SHA256, runtime.GOARCH, true) == nil {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := stageTransport(name, filepath.Dir(dest))
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if exists {
		if err := safefile.Copy(dest, dest+".bak", 0755); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	return safefile.SyncDir(filepath.Dir(dest))
}
