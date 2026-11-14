package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func panelUnit(name string) bool {
	return (strings.HasPrefix(name, "rovunet-") || strings.HasPrefix(name, "slipgate-")) &&
		(strings.HasSuffix(name, ".service") || strings.HasSuffix(name, ".timer")) && filepath.Base(name) == name
}

// RemovePanelUnits stops timers as well as workers, then removes their unit
// files and enablement links. A failed stop cannot lead to deletion of data
// still being written by a worker.
func RemovePanelUnits() error {
	return removePanelUnits(systemdDir)
}

func removePanelUnits(dir string) error {
	names := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && panelUnit(d.Name()) {
			names[d.Name()] = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Include loaded units whose files were removed by an older uninstaller.
	output, err := exec.Command("systemctl", "list-units", "--all", "--plain", "--no-legend", "rovunet-*", "slipgate-*").Output()
	if err != nil {
		return fmt.Errorf("enumerate loaded panel units: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && panelUnit(fields[0]) {
			names[fields[0]] = true
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if strings.HasSuffix(ordered[i], ".timer") != strings.HasSuffix(ordered[j], ".timer") {
			return strings.HasSuffix(ordered[i], ".timer")
		}
		return ordered[i] < ordered[j]
	})
	for _, name := range ordered {
		stop := runQuiet("systemctl", "stop", name)
		if stop != nil && runQuiet("systemctl", "is-active", "--quiet", name) == nil {
			return fmt.Errorf("panel unit still active; files retained: %s", name)
		}
		if err := runQuiet("systemctl", "disable", name); err != nil {
			if _, e := os.Lstat(filepath.Join(dir, name)); e == nil {
				return fmt.Errorf("disable panel unit %s: %w", name, err)
			}
		}
	}
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && names[d.Name()] {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := daemonReload(); err != nil {
		return err
	}
	for _, name := range ordered {
		_ = runQuiet("systemctl", "reset-failed", name)
	}
	return nil
}
