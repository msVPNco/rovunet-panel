package binary

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

type InstalledOwnership struct {
	Schema int               `json:"schema"`
	Files  map[string]string `json:"files"`
}

var ownershipPath = filepath.Join(config.DefaultConfigDir, "installed-files.json")
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ownedBinaryPath(path string) bool {
	if filepath.Dir(path) != config.DefaultBinDir {
		return false
	}
	name := filepath.Base(path)
	if name == ".rovunet-update-state.json" || name == ".rovunet-update.lock" || name == ".rovunet-install-state.json" {
		return true
	}
	for _, base := range []string{"rovunet", "xray", "rovunet-xray", "rovunet-amneziawg-go", "rovunet-awg", "rovunet-sing-box", "rovunet-mita", "rovunet-awg-provenance.json", "dnstt-server", "slipstream-server", "slipstream-client", "vaydns-server", "caddy-naive", "microsocks"} {
		if name == base || name == base+".bak" || name == base+".provenance.json" {
			return true
		}
		if strings.HasPrefix(name, base+".before-") && digestPattern.MatchString(strings.TrimPrefix(name, base+".before-")) {
			return true
		}
	}
	return false
}

func ReadInstalledOwnership() (InstalledOwnership, error) {
	v := InstalledOwnership{Schema: 1, Files: map[string]string{}}
	info, err := os.Lstat(ownershipPath)
	if os.IsNotExist(err) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return v, errors.New("unsafe installed-file receipt")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Uid != 0 {
		return v, errors.New("installed-file receipt is not root-owned")
	}
	b, err := os.ReadFile(ownershipPath)
	if err != nil {
		return v, err
	}
	if len(b) > 1<<20 || json.Unmarshal(b, &v) != nil || v.Schema != 1 || v.Files == nil {
		return v, errors.New("invalid installed-file receipt")
	}
	for path, hash := range v.Files {
		if !ownedBinaryPath(path) || !digestPattern.MatchString(hash) {
			return v, errors.New("invalid installed-file ownership binding")
		}
	}
	return v, nil
}

// RecordOwnedFile is called only after this panel commits a verified artifact
// or writes its own receipt. An arbitrary similarly named file is not adopted.
func RecordOwnedFile(path string) error {
	if !ownedBinaryPath(path) {
		return nil
	} // isolated test destinations are not installation paths
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("owned file is not regular: %s", path)
	}
	hash, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(ownershipPath), 0750); err != nil {
		return err
	}
	lock, err := os.OpenFile(ownershipPath+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	v, err := ReadInstalledOwnership()
	if err != nil {
		return err
	}
	v.Files[path] = hash
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return safefile.Write(ownershipPath, b, 0600)
}

// RecordOwnedBackup records a recovery copy only when the previous artifact
// was already owned with that exact digest. Third-party backups stay unknown.
func RecordOwnedBackup(target, backup, previous string) error {
	if !ownedBinaryPath(target) {
		return nil
	}
	v, err := ReadInstalledOwnership()
	if err != nil {
		return err
	}
	if v.Files[target] != previous {
		return nil
	}
	actual, err := FileSHA256(backup)
	if err != nil {
		return err
	}
	if actual != previous {
		return errors.New("owned backup digest mismatch")
	}
	return RecordOwnedFile(backup)
}
