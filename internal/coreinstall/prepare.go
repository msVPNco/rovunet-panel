package coreinstall

import (
	"fmt"
	"path/filepath"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

// Prepare is a systemd ExecCondition: regenerate from current account state
// before a boot/restart, preventing expired or exhausted credentials from
// returning just because their engine starts before the limits timer.
func Prepare(name string, users []config.UserConfig) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	o, st, managed, err := loadUserCore(name)
	if err != nil {
		return err
	}
	if !managed {
		return nil
	}
	if st.Disabled {
		return fmt.Errorf("core is administratively stopped")
	}
	dir := filepath.Join(Root, name)
	if _, err := writeUserCore(dir, o, &st, users); err != nil {
		return err
	}
	if st.Paused {
		return fmt.Errorf("core has no active eligible account")
	}
	return TestConfig(filepath.Join(dir, "server.json"))
}
