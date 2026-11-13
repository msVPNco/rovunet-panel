package handlers

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtraAddressPersistentIDs(t *testing.T) {
	c := subTestConfig()
	c.ExtraAddresses = []string{"198.51.100.1", "same.example", "198.51.100.2"}
	path := filepath.Join(t.TempDir(), "cfg")
	ok16(t, c.SaveTo(path))
	// Snapshot emitted before the mapping existed: migration keeps all suffixes.
	if !c.EnsureAddressIdentities() {
		t.Fatal("not migrated")
	}
	before := map[string]string{}
	for i, a := range c.ExtraAddresses {
		before[a] = c.AddressIdentity(a, i)
	}
	if before["same.example"] != "ip3" {
		t.Fatal(before)
	}
	ok16(t, c.Save())
	ok16(t, setPublicAddress(c, c.PublicAddress+",198.51.100.2,same.example,198.51.100.1,203.0.113.99"))
	for i, a := range c.ExtraAddresses {
		if id, ok := before[a]; ok && c.AddressIdentity(a, i) != id {
			t.Fatal("reorder changed identity")
		}
	}
	newID := c.AddressIdentity("203.0.113.99", 3)
	if newID != "ip5" {
		t.Fatal(newID)
	}
	ok16(t, setPublicAddress(c, c.PublicAddress+",same.example,203.0.113.99"))
	ok16(t, setPublicAddress(c, c.PublicAddress+",new.example,198.51.100.1,SAME.EXAMPLE"))
	if c.AddressIdentity("new.example", 0) != "ip6" || c.AddressIdentity("198.51.100.1", 1) != "ip2" || c.AddressIdentity("same.example", 2) != "ip3" {
		t.Fatal("tombstone or canonical alias lost")
	}
	fresh, e := config.LoadFrom(path)
	ok16(t, e)
	if fresh.AddressIdentity("SAME.EXAMPLE.", 0) != "ip3" {
		t.Fatal("persisted alias lost")
	}
	// Two endpoints with the same display label still obtain distinct suffixes.
	if fresh.AddressIdentity("new.example", 0) == fresh.AddressIdentity("same.example", 1) {
		t.Fatal("name collision")
	}
	for _, id := range fresh.ExtraAddressIDs {
		if !strings.HasPrefix(id, "ip") {
			t.Fatal(id)
		}
	}
}
