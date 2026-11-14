package handlers

import (
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
)

func RotateSubscriptionLink(cfg *config.Config, username string, out actions.OutputWriter) error {
	return RotateSubscriptionOperation(cfg, username, "")
}

// Blank selects one repeatable default operation per installation and username. A further
// deliberate replacement in that period requires a NEW caller-supplied ID.
func RotateSubscriptionOperation(cfg *config.Config, user, id string) error {
	return withPublicationLock(func() error {
		if e := cfg.RefreshIfChanged(); e != nil {
			return e
		}
		if subscriptionHost(cfg) == "" {
			return errors.New("enabled HTTPS subscription endpoint required")
		}
		if e := cfg.ValidateSubscriptionTokens(); e != nil {
			return e
		}
		if id == "" {
			u := cfg.GetUser(user)
			if u == nil {
				return errSuperseded
			}
			id = rotationRequest(cfg.ServerID, user, "", "", "default")[:32]
		}
		if !tokenOK(id) {
			return errors.New("operation ID must be 32 lowercase hex digits")
		}
		// Always finish recovery/owned GC even if the terminal receipt exists.
		if _, e := resumeRotation(cfg); e != nil && !errors.Is(e, errSuperseded) {
			return e
		}
		if hit, e := checkReceipt(cfg, user, id); hit || e != nil {
			return e
		}
		return beginRotation(cfg, user, id)
	})
}
