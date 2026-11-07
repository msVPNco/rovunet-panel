package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

// A subscription is a static JSON file the app downloads over HTTPS:
//
//	{"version":1,"name":"<user>","items":[{"id":"<tag>","link":"rovunet://..."}]}
//
// This is the only shape the app accepts (it rejects any other key). The
// file holds exactly the rovunet:// configs `users configs` prints for the
// user, so the two can never disagree, and is served by the NaïveProxy
// Caddy on its port (443) at /sub/<token>.json.

type subItem struct {
	ID   string `json:"id"`
	Link string `json:"link"`
}

type subFeed struct {
	Version int       `json:"version"`
	Name    string    `json:"name"`
	Items   []subItem `json:"items"`
}

// captureOutput collects printed lines.
type captureOutput struct{ lines []string }

func (c *captureOutput) Info(string)    {}
func (c *captureOutput) Success(string) {}
func (c *captureOutput) Warning(string) {}
func (c *captureOutput) Error(string)   {}
func (c *captureOutput) Print(m string) { c.lines = append(c.lines, m) }

var subTagRe = regexp.MustCompile(`^\[([^\]]+)\]`)

// subscriptionItems turns the lines `users configs` prints for one user into
// feed items: each rovunet:// line, named by the [tag] header above it.
func subscriptionItems(lines []string) []subItem {
	var items []subItem
	tag := ""
	used := map[string]int{}
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if m := subTagRe.FindStringSubmatch(t); m != nil {
			tag = m[1]
			continue
		}
		if !strings.HasPrefix(t, "rovunet://") {
			continue
		}
		id := tag
		if id == "" {
			id = "config"
		}
		used[id]++
		if used[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, used[id])
		}
		items = append(items, subItem{ID: id, Link: t})
	}
	return items
}

// subscriptionHost is the NaïveProxy domain (and port) that serves the files,
// or "" when no NaïveProxy tunnel exists.
func subscriptionHost(cfg *config.Config) string {
	for _, t := range cfg.Tunnels {
		if t.Transport == config.TransportNaive && t.Enabled && t.Domain != "" {
			if t.Naive != nil && t.Naive.Port != 0 && t.Naive.Port != 443 {
				return fmt.Sprintf("%s:%d", t.Domain, t.Naive.Port)
			}
			return t.Domain
		}
	}
	return ""
}

// SubscriptionURL is the link a user pastes into the app, or "".
func SubscriptionURL(cfg *config.Config, u config.UserConfig) string {
	host := subscriptionHost(cfg)
	if host == "" || u.SubToken == "" {
		return ""
	}
	return fmt.Sprintf("https://%s/sub/%s.json", host, u.SubToken)
}

// buildFeed renders one user's feed.
func buildFeed(cfg *config.Config, u config.UserConfig) ([]byte, error) {
	c := &captureOutput{}
	printUserConfigs(cfg, u.Username, u.Password, c)
	items := subscriptionItems(c.lines)
	if len(items) == 0 {
		return nil, nil
	}
	items = addExtraAddressItems(cfg, u, items)
	return json.MarshalIndent(subFeed{Version: 1, Name: u.Username, Items: items}, "", "  ")
}

// WriteSubscriptions brings the subscription files in line with the users:
// one file per user, none for removed users or old tokens. It does nothing
// without a NaïveProxy tunnel. Users without a token get one (and the config
// is saved). Call it after any change to users or tunnels.
func WriteSubscriptions(cfg *config.Config, out actions.OutputWriter) error {
	if subscriptionHost(cfg) == "" {
		return nil
	}
	minted := false
	for i := range cfg.Users {
		if cfg.Users[i].SubToken == "" {
			cfg.Users[i].SubToken = config.NewSubToken()
			minted = true
		}
	}
	if minted {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save subscription tokens: %w", err)
		}
	}
	root := transport.SubscriptionDir
	dir := filepath.Join(root, "sub")
	// Directories can be entered but not listed, so a local account (SSH users
	// are real accounts) cannot enumerate the tokens; files are world-readable
	// because Caddy may run as another user.
	for _, d := range []string{filepath.Dir(root), root, dir} {
		if err := os.MkdirAll(d, 0711); err != nil {
			return err
		}
		_ = os.Chmod(d, 0711)
	}
	keep := map[string]bool{}
	var firstErr error
	for _, u := range cfg.Users {
		body, err := buildFeed(cfg, u)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if body == nil {
			continue
		}
		name := u.SubToken + ".json"
		keep[name] = true
		path := filepath.Join(dir, name)
		if old, err := os.ReadFile(path); err == nil && string(old) == string(body) {
			continue
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, body, 0644); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.Rename(tmp, path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !keep[e.Name()] {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return firstErr
}

// addExtraAddressItems repeats the configs that connect by the server's
// address once per extra address. Configs that don't (domain-based ones such
// as NaïveProxy) come out identical and are skipped, so only the address-based
// ones are added, with the address number in their id.
func addExtraAddressItems(cfg *config.Config, u config.UserConfig, items []subItem) []subItem {
	have := map[string]bool{}
	ids := map[string]bool{}
	for _, it := range items {
		have[it.Link] = true
		ids[it.ID] = true
	}
	for n, addr := range cfg.ExtraAddresses {
		alt := *cfg
		alt.PublicAddress = addr
		c := &captureOutput{}
		printUserConfigs(&alt, u.Username, u.Password, c)
		for _, it := range subscriptionItems(c.lines) {
			if have[it.Link] {
				continue
			}
			have[it.Link] = true
			id := fmt.Sprintf("%s-ip%d", it.ID, n+2)
			for ids[id] {
				id += "x"
			}
			ids[id] = true
			items = append(items, subItem{ID: id, Link: it.Link})
		}
	}
	return items
}
