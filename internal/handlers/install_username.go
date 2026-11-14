package handlers

import (
	"fmt"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/system"
)

func checkInstallUsername(cfg *config.Config, name string, exists func(string) bool) error {
	if err := config.ValidateUsername(name); err != nil {
		return err
	}
	if cfg.GetUser(name) != nil || exists(name) {
		return fmt.Errorf("Name %q is already used by a panel or Linux account. Choose another name; its password and permissions were not changed", name)
	}
	return nil
}

func askAvailableInstallUsername(cfg *config.Config, out actions.OutputWriter) (string, error) {
	for {
		name, err := prompt.String("Username", "user1")
		if err != nil {
			return "", err
		}
		if err := checkInstallUsername(cfg, name, system.UserExists); err != nil {
			out.Warning(err.Error())
			continue
		}
		return name, nil
	}
}
