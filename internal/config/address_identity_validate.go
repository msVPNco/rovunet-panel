package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Ambiguous historical spellings need an explicit reconcile. Refuse to
// silently merge already-published positional IDs or transfer a saved pin.
func ValidateAddressSet(main string, extra []string) error {
	seen := map[string]string{}
	if main != "" {
		seen[canonicalAddress(main)] = main
	}
	for _, a := range extra {
		key := canonicalAddress(a)
		if old, exists := seen[key]; exists && old != a {
			return fmt.Errorf("address aliases %q and %q require explicit reconciliation", old, a)
		}
		seen[key] = a
	}
	return nil
}
func (c *Config) ValidateAddressIdentities() error {
	if e := ValidateAddressSet(c.PublicAddress, c.ExtraAddresses); e != nil {
		return e
	}
	seen := map[string]string{}
	for address, id := range c.ExtraAddressIDs {
		n, e := strconv.Atoi(strings.TrimPrefix(id, "ip"))
		if e != nil || n < 2 || id != fmt.Sprintf("ip%d", n) {
			return fmt.Errorf("invalid persisted address identity")
		}
		if old, exists := seen[id]; exists && old != address {
			return fmt.Errorf("address identity collision requires reconciliation")
		}
		seen[id] = address
	}
	return nil
}
