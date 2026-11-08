package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
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
// one subscription and one status file per user, only the status file for a
// stopped user, nothing for removed users or old tokens. It does nothing
// without a NaïveProxy tunnel. A missing token or identifier is made (and the
// config is saved). Call it after any change to users or tunnels, and after
// each limits check, which refreshes the traffic in the status files.
func WriteSubscriptions(cfg *config.Config, out actions.OutputWriter) error {
	if subscriptionHost(cfg) == "" {
		return nil
	}
	minted := mintIDs(cfg)
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
	write := func(name string, body []byte) {
		keep[name] = true
		path := filepath.Join(dir, name)
		if old, err := os.ReadFile(path); err == nil && string(old) == string(body) {
			return
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, body, 0644); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		if err := os.Rename(tmp, path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	status := func(u config.UserConfig, reason string, in statusInput) {
		if u.SubToken == "" {
			return
		}
		name := u.SubToken + statusSuffix
		body, err := statusBody(buildStatus(in, u, reason), filepath.Join(dir, name))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		write(name, body)
	}
	in := newStatusInput(cfg, time.Now())
	for _, u := range cfg.Users {
		status(u, "", in)
	}
	// A stopped user keeps the status file, so the app can say why, but not
	// the subscription: those configs no longer work.
	for _, a := range cfg.Archived {
		if cfg.GetUser(a.Username) == nil {
			status(a.UserConfig, a.Reason, in)
		}
	}
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
		write(u.SubToken+".json", body)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !keep[e.Name()] {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return firstErr
}

// mintIDs gives the server, and every user and stopped user, whatever
// identifier it lacks (configs from before they existed), and reports
// whether anything was added.
func mintIDs(cfg *config.Config) bool {
	minted := false
	mint := func(s *string, f func() string) {
		if *s == "" {
			*s = f()
			minted = true
		}
	}
	mint(&cfg.ServerID, config.NewID)
	for i := range cfg.Users {
		u := &cfg.Users[i]
		mint(&u.SubToken, config.NewSubToken)
		mint(&u.AccountID, config.NewID)
		mint(&u.PeriodID, config.NewID)
	}
	for i := range cfg.Archived {
		a := &cfg.Archived[i]
		mint(&a.AccountID, config.NewID)
		mint(&a.PeriodID, config.NewID)
	}
	return minted
}

// addExtraAddressItems repeats the configs that connect by the server's
// address once per extra address. Configs that don't (domain-based ones such
// as NaïveProxy) come out identical and are skipped, so only the address-based
// ones are added, with the address number in their id.
func addExtraAddressItems(cfg *config.Config, u config.UserConfig, items []subItem) []subItem {
	have := map[string]bool{}
	ids := map[string]bool{}
	for _, it := range items {
		have[linkIdentity(it.Link)] = true
		ids[it.ID] = true
	}
	main := cfg.PublicAddress
	defer func() { cfg.PublicAddress = main }()
	for n, addr := range cfg.ExtraAddresses {
		// A Config holds a lock and can't be copied, so the address is
		// swapped for the print and put back.
		cfg.PublicAddress = addr
		c := &captureOutput{}
		printUserConfigs(cfg, u.Username, u.Password, c)
		for _, it := range subscriptionItems(c.lines) {
			if have[linkIdentity(it.Link)] {
				continue
			}
			have[linkIdentity(it.Link)] = true
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

// linkIdentity is a link without the fields that only name its group (the
// collection id and name, which are made from the server address). Two links
// with the same identity connect to the same place the same way.
func linkIdentity(link string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "rovunet://"))
	if err != nil {
		return link
	}
	fields := strings.Split(string(raw), "|")
	for _, i := range []int{clientcfg.FCollectionID, clientcfg.FCollectionName} {
		if i < len(fields) {
			fields[i] = ""
		}
	}
	return strings.Join(fields, "|")
}
