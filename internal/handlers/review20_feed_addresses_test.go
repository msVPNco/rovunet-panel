package handlers

import (
	"encoding/json"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"path/filepath"
	"testing"
)

func TestExtraAddressFeedSnapshot(t *testing.T) {
	c := subTestConfig()
	c.Tunnels = append(c.Tunnels, config.TunnelConfig{Tag: "ssh", Transport: config.TransportSSH, Backend: config.BackendSSH, Enabled: true})
	c.ExtraAddresses = []string{"198.51.100.1", "198.51.100.2"}
	ok16(t, c.SaveTo(filepath.Join(t.TempDir(), "config.json")))
	read := func() map[string]string {
		raw, e := buildFeed(c, c.Users[0])
		ok16(t, e)
		var f subFeed
		ok16(t, json.Unmarshal(raw, &f))
		out := map[string]string{}
		for _, item := range f.Items {
			if _, exists := out[item.ID]; exists {
				t.Fatal("duplicate item ID")
			}
			out[item.ID] = item.Link
		}
		return out
	}
	before := read() // actual positional feed from before mapping existed
	if before["ssh-ip2"] == "" || before["ssh-ip3"] == "" {
		t.Fatal("fixture expected old emitted SSH tags")
	}
	c.EnsureAddressIdentities()
	ok16(t, c.Save())
	ok16(t, setPublicAddress(c, c.PublicAddress+",198.51.100.2,198.51.100.1"))
	reordered := read()
	if len(before) != len(reordered) {
		t.Fatal("reorder count")
	}
	for id, link := range before {
		if reordered[id] != link {
			t.Fatalf("old snapshot changed at %s", id)
		}
	}
	ok16(t, setPublicAddress(c, c.PublicAddress+",198.51.100.1,203.0.113.99"))
	changed := read()
	if changed["ssh-ip3"] != "" {
		t.Fatal("new endpoint inherited removed identity")
	}
	if changed["ssh-ip4"] == "" {
		t.Fatal("new endpoint needs new identity")
	}
	if changed["ssh-ip2"] != before["ssh-ip2"] {
		t.Fatal("existing endpoint changed")
	}
}

func TestAmbiguousAddressAliasesNeedReconcile(t *testing.T) {
	c := subTestConfig()
	c.ExtraAddresses = []string{"same.example", "SAME.EXAMPLE"}
	if _, e := buildFeed(c, c.Users[0]); e == nil {
		t.Fatal("ambiguous legacy IDs were silently merged")
	}
	c.ExtraAddresses = []string{"198.51.100.1", "198.51.100.2"}
	c.ExtraAddressIDs = map[string]string{"198.51.100.1": "ip2", "198.51.100.2": "ip2"}
	if _, e := buildFeed(c, c.Users[0]); e == nil {
		t.Fatal("persisted identity collision accepted")
	}
}
