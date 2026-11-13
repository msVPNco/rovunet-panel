package handlers

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

// RotateSubscriptionLink changes only the fetch capability. Existing runtime
// credentials are deliberately not described as revoked by a URL change.
func RotateSubscriptionLink(cfg *config.Config, username string, out actions.OutputWriter) error {
	if subscriptionHost(cfg) == "" {
		return errors.New("an enabled HTTPS subscription endpoint is required")
	}
	if err := cfg.ValidateSubscriptionTokens(); err != nil {
		return err
	}
	u := cfg.GetUser(username)
	if u == nil {
		return errors.New("active account not found")
	}
	previous := *u
	next := previous
	next.SubToken = config.NewSubToken()
	cfg.AddUser(next)
	if err := cfg.Save(); err != nil {
		cfg.AddUser(previous)
		return err
	}
	var problems []error
	if previous.SubToken != "" {
		for _, suffix := range []string{".json", ".windows.json", ".status.json", ".windows.status.json"} {
			err := os.Remove(filepath.Join(transport.SubscriptionDir, "sub", previous.SubToken+suffix))
			if err != nil && !os.IsNotExist(err) {
				problems = append(problems, errors.New("old subscription file could not be retired; retry subscription refresh"))
			}
		}
	}
	if err := WriteSubscriptions(cfg, out); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}
