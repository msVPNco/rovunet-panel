package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

// A subscription is a static JSON file the app downloads over HTTPS:
//
//	{"version":1,"name":"<user>","items":[{"id":"<tag>","link":"rovunet://..."}]}
//
// This is the only shape the app accepts (it rejects any other key). The
// file holds the same per-account configurations as `users configs`, with
// two lossless standard URI encodings for older readers. It is served by NaïveProxy
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

var subscriptionWriteMu sync.Mutex

func userSubscriptionItems(cfg *config.Config, u config.UserConfig) ([]subItem, error) {
	return subscriptionItemsForClient(cfg, u, "")
}

func subscriptionItemsForClient(cfg *config.Config, u config.UserConfig, client string) ([]subItem, error) {
	profiles, err := clientexport.Build(cfg, u, cfg.Tunnels, clientexport.Options{Client: client, All: showAllConfigs(), CompatibleSubscription: true, StatsWorking: statsWorking})
	var items []subItem
	for _, p := range profiles {
		items = append(items, subItem{ID: p.ID, Link: p.Link})
	}
	return items, err
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

func WindowsSubscriptionURL(cfg *config.Config, u config.UserConfig) string {
	link := SubscriptionURL(cfg, u)
	if link == "" {
		return ""
	}
	return strings.TrimSuffix(link, ".json") + ".windows.json"
}

// buildFeed renders one user's feed.
func buildFeed(cfg *config.Config, u config.UserConfig) ([]byte, error) {
	return buildFeedForClient(cfg, u, "")
}

func buildFeedForClient(cfg *config.Config, u config.UserConfig, client string) ([]byte, error) {
	if e := cfg.ValidateAddressIdentities(); e != nil {
		return nil, e
	}
	items, err := subscriptionItemsForClient(cfg, u, client)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	items, err = extraAddressItemsForClient(cfg, u, items, client)
	if err != nil {
		return nil, err
	}
	if len(items) > 1000 {
		return nil, errors.New("subscription exceeds the strict v1 reader's 1000-item limit; reduce extra addresses; previous feed retained")
	}
	return json.MarshalIndent(subFeed{Version: 1, Name: u.Username, Items: items}, "", "  ")
}

// WriteSubscriptions brings the subscription files in line with the users:
// one subscription and one status file per user, only the status file for a
// stopped user, nothing for removed users or old tokens. It does nothing
// without a NaïveProxy tunnel. A missing token or identifier is made (and the
// config is saved). Call it after any change to users or tunnels, and after
// each limits check, which refreshes the traffic in the status files.
func WriteSubscriptions(cfg *config.Config, out actions.OutputWriter) error {
	if err := cfg.ValidateSubscriptionTokens(); err != nil {
		return err
	}
	if subscriptionHost(cfg) == "" {
		return nil
	}
	return withPublicationLock(func() error {
		if err := cfg.RefreshIfChanged(); err != nil {
			return err
		}
		if err := resumeCarry(cfg); err != nil {
			return err
		}
		if _, err := resumeRotation(cfg); err != nil && !errors.Is(err, errSuperseded) {
			return err
		}
		return writeCurrentSubscriptions(cfg, out)
	})
}

func withPublicationLock(work func() error) error {
	// The limits timer and administrative commands can refresh concurrently.
	// Readers keep seeing the previous atomic files while a writer works.
	subscriptionWriteMu.Lock()
	defer subscriptionWriteMu.Unlock()
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
	lock, err := os.OpenFile(filepath.Join(root, ".write.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return work()
}

func writeCurrentSubscriptions(cfg *config.Config, out actions.OutputWriter) error {
	if err := cfg.RefreshIfChanged(); err != nil {
		return fmt.Errorf("could not refresh current account state; previous subscription files retained: %w", err)
	}
	if err := cfg.ValidateSubscriptionTokens(); err != nil {
		return err
	}
	if e := cfg.ValidateAddressIdentities(); e != nil {
		return e
	}
	if mintIDs(cfg) {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save subscription tokens: %w", err)
		}
	}
	// Stage a complete copy; status revisions are continued from that copy.
	root := transport.SubscriptionDir
	stage, err := os.MkdirTemp(root, ".refresh-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	dir := filepath.Join(stage, "next")
	if err = copyGeneration(publicDir(), dir); err != nil {
		return err
	}
	renderErr := renderCurrentInto(cfg, out, dir)
	if renderErr != nil {
		// Discard partial renders, retain usable prior feeds for authorized users,
		// and still remove revoked capabilities in one generation.
		dir = filepath.Join(stage, "safe")
		if err = copyGeneration(publicDir(), dir); err != nil {
			return errors.Join(renderErr, err)
		}
		allowed := map[string]bool{}
		for _, u := range cfg.Users {
			for i, ext := range aliasSuffixes {
				if (i >= 2 && ext != trustSuffix) || rotationEligible(cfg, u) {
					allowed[u.SubToken+ext] = true
				}
			}
		}
		for _, v := range cfg.Archived {
			if cfg.GetUser(v.Username) == nil {
				for _, ext := range aliasSuffixes[2:] {
					if ext != trustSuffix {
						allowed[v.SubToken+ext] = true
					}
				}
			}
		}
		entries, e := os.ReadDir(dir)
		if e != nil {
			return e
		}
		for _, f := range entries {
			if !allowed[f.Name()] {
				if e = os.Remove(filepath.Join(dir, f.Name())); e != nil {
					return e
				}
			}
		}
	}
	apply := func() error {
		old, e := treeDigest(publicDir())
		if e != nil {
			return e
		}
		next, e := treeDigest(dir)
		if e != nil {
			return e
		}
		if e = safefile.SyncDir(dir); e != nil {
			return e
		}
		if e = safefile.SyncDir(stage); e != nil {
			return e
		}
		return swapTo(publicDir(), dir, old, next)
	}
	if cfg.Path() == "" {
		return errors.Join(renderErr, apply())
	} // in-memory fixture
	return errors.Join(renderErr, cfg.WithCurrentFile(apply))
}
func renderCurrentInto(cfg *config.Config, out actions.OutputWriter, dir string) error {
	keep := map[string]bool{}
	var firstErr error
	write := func(name string, body []byte) {
		keep[name] = true
		path := filepath.Join(dir, name)
		if old, err := os.ReadFile(path); err == nil && string(old) == string(body) {
			return
		}
		if err := safefile.Write(path, body, 0644); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
	}
	status := func(u config.UserConfig, reason string, in statusInput, suffix string) {
		if u.SubToken == "" {
			return
		}
		name := u.SubToken + suffix + statusSuffix
		st := buildStatus(in, u, reason)
		if u.SQLiteMeter != nil && st.UsedBytes == nil {
			return
		}
		keep[name] = true
		body, err := statusBody(st, filepath.Join(dir, name))
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
		status(u, "", in, "")
		status(u, "", in, ".windows")
	}
	// A stopped user keeps the status file, so the app can say why, but not
	// the subscription: those configs no longer work.
	for _, a := range cfg.Archived {
		if cfg.GetUser(a.Username) == nil {
			status(a.UserConfig, a.Reason, in, "")
			status(a.UserConfig, a.Reason, in, ".windows")
		}
	}
	for _, u := range cfg.Users {
		if !rotationEligible(cfg, u) {
			continue
		}
		trust, e := buildSubscriptionTrust(cfg, u)
		if e != nil {
			if firstErr == nil {
				firstErr = e
			}
		} else {
			write(u.SubToken+trustSuffix, trust)
		}
		for _, client := range []string{"", "windows"} {
			suffix := ""
			if client == "windows" {
				suffix = ".windows"
			}
			name := u.SubToken + suffix + ".json"
			body, err := buildFeedForClient(cfg, u, client)
			if err != nil {
				// A transient provisioning/read error must not turn a valid
				// existing subscription into a 404. Explicit removals below
				// still retire their old URLs and credentials as before.
				keep[name] = true
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if body == nil {
				continue
			}
			write(name, body)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.Join(firstErr, err)
	}
	for _, e := range entries {
		if !keep[e.Name()] {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// mintIDs gives the server, and every user and stopped user, whatever
// identifier it lacks (configs from before they existed), and reports
// whether anything was added.
func mintIDs(cfg *config.Config) bool {
	minted := cfg.EnsureAddressIdentities()
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

// extraAddressItemsForClient repeats configs that connect by the server's
// address once per extra address. Configs that don't (domain-based ones such
// as NaïveProxy) come out identical and are skipped, so only the address-based
// ones are added, with the address number in their id.
func extraAddressItemsForClient(cfg *config.Config, u config.UserConfig, items []subItem, client string) ([]subItem, error) {
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
		extra, err := subscriptionItemsForClient(cfg, u, client)
		if err != nil {
			return nil, err
		}
		for _, it := range extra {
			if have[linkIdentity(it.Link)] {
				continue
			}
			have[linkIdentity(it.Link)] = true
			id := fmt.Sprintf("%s-%s", it.ID, cfg.AddressIdentity(addr, n))
			for ids[id] {
				id += "x"
			}
			ids[id] = true
			items = append(items, subItem{ID: id, Link: it.Link})
		}
	}
	return items, nil
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
