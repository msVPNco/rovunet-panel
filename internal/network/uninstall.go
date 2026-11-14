package network

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

// RemoveResolvedStubDropin removes only the exact generated override. The
// administrator's resolv.conf, static DNS and other dropins remain untouched;
// resolved's uplink symlink continues to work with its stub enabled again.
func RemoveResolvedStubDropin() error {
	removed, err := removeResolvedStubDropin("/etc/systemd/resolved.conf.d/rovunet-no-stub.conf")
	if err != nil || !removed {
		return err
	}
	if exec.Command("systemctl", "is-active", "--quiet", "systemd-resolved").Run() == nil {
		return run("systemctl", "restart", "systemd-resolved")
	}
	return nil
}

func removeResolvedStubDropin(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("non-regular resolver override retained for review")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(b, []byte("[Resolve]\nDNSStubListener=no\n")) {
		return false, fmt.Errorf("custom resolver override retained for review")
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, safefile.SyncDir(filepath.Dir(path))
}
