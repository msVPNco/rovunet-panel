package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func canonicalAddress(s string) string {
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return s
}

// Preserve the published positional suffix on first migration, retaining
// tombstones forever so a new endpoint cannot silently inherit an old pin.
func (c *Config) EnsureAddressIdentities() bool {
	if len(c.ExtraAddresses) == 0 && len(c.ExtraAddressIDs) == 0 {
		return false
	}
	changed := false
	if c.ExtraAddressIDs == nil {
		c.ExtraAddressIDs = map[string]string{}
		changed = true
	}
	max := 1
	for _, id := range c.ExtraAddressIDs {
		if n, e := strconv.Atoi(strings.TrimPrefix(id, "ip")); e == nil && n > max {
			max = n
		}
	}
	initial := len(c.ExtraAddressIDs) == 0
	for i, address := range c.ExtraAddresses {
		key := canonicalAddress(address)
		if _, ok := c.ExtraAddressIDs[key]; ok {
			continue
		}
		n := max + 1
		if initial {
			n = i + 2
		}
		c.ExtraAddressIDs[key] = fmt.Sprintf("ip%d", n)
		if n > max {
			max = n
		}
		changed = true
	}
	return changed
}
func (c *Config) AddressIdentity(address string, index int) string {
	if id := c.ExtraAddressIDs[canonicalAddress(address)]; id != "" {
		return id
	}
	return fmt.Sprintf("ip%d", index+2) // pre-migration in-memory fixture
}
