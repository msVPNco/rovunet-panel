package binary

import (
	"fmt"
	"os"
)

// RecoverInterrupted repairs what an update that was cut short leaves behind
// for the binary at binPath. An update moves the old binary to binPath+".bak",
// downloads the new one to binPath+".tmp" and puts it in place; if the program
// stops in between (a dead connection, a killed process, a reboot), the
// service is left with no binary and every start fails (systemd status
// 203/EXEC). This puts the old binary back when the binary is missing and a
// backup exists, and removes a partial download. It reports whether it
// restored the binary. It never touches a binary that is in place, and never
// removes a backup.
func RecoverInterrupted(binPath string) (restored bool, err error) {
	if err := os.Remove(binPath + ".tmp"); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("remove partial download: %w", err)
	}
	if _, err := os.Stat(binPath); err == nil {
		return false, nil // in place
	} else if !os.IsNotExist(err) {
		return false, err
	}
	bak := binPath + ".bak"
	if st, err := os.Stat(bak); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return false, nil // nothing usable to put back
	}
	if err := os.Rename(bak, binPath); err != nil {
		return false, fmt.Errorf("restore %s: %w", binPath, err)
	}
	return true, nil
}
