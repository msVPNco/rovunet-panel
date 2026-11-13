package config

import (
	"encoding/hex"
	"errors"
)

func ValidSubscriptionToken(token string) bool {
	b, err := hex.DecodeString(token)
	return err == nil && len(b) == 16 && len(token) == 32
}

// Empty legacy tokens are minted later. Nonempty tokens are filesystem names
// and must never contain a path, separator, extension or arbitrary suffix.
func (c *Config) ValidateSubscriptionTokens() error {
	for _, u := range c.Users {
		if u.SubToken != "" && !ValidSubscriptionToken(u.SubToken) {
			return errors.New("invalid subscription token; filesystem paths are not allowed")
		}
	}
	for _, u := range c.Archived {
		if u.SubToken != "" && !ValidSubscriptionToken(u.SubToken) {
			return errors.New("invalid archived subscription token; filesystem paths are not allowed")
		}
	}
	return nil
}
