// Package clientexport produces structured profiles. CLI presentation and JSON
// subscriptions consume these records; neither parses the other's stdout.
package clientexport

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/protocols"
)

type Profile struct {
	ID, Protocol, Link, Standard string
	Rank                         int
	File, NoLogin                bool
}

type Options struct {
	Format                 string
	Client                 string
	All                    bool
	CompatibleSubscription bool
	StatsWorking           func(string) bool
}

// Build returns only enabled, account-eligible profiles. IDs retain the tags
// used by the existing feed, independent of headings, display order and URIs.
func Build(cfg *config.Config, user config.UserConfig, tunnels []config.TunnelConfig, opts Options) ([]Profile, error) {
	if opts.Client != "" && opts.Client != "android" && opts.Client != "windows" {
		return nil, fmt.Errorf("unknown client platform")
	}
	if user.Expired(time.Now()) {
		return nil, nil
	}
	if opts.Format == "" {
		opts.Format = "rovunet"
	}
	if opts.StatsWorking == nil {
		opts.StatsWorking = coreinstall.StatsWorking
	}
	var profiles []Profile
	var errs []error
	if user.Username != "" && (opts.Format == "auto" || opts.Format == "rovunet" || opts.Format == "standard") {
		cores, err := coreinstall.Profiles(user)
		if err != nil {
			errs = append(errs, err)
		}
		for _, p := range cores {
			if user.SQLiteMeter == nil && user.MeteredOnly() && !opts.StatsWorking(p.Core) {
				continue
			}
			file := p.Protocol == "wireguard" || p.Protocol == "amneziawg"
			if file && opts.Format == "standard" {
				continue
			}
			link := p.Link
			// Older Windows/macOS readers skip unsupported standard profiles, but reject
			// the entire subscription for unsupported native xray envelopes.
			// Android accepts both lossless forms. Keep IDs, credentials and
			// native envelopes for CLI exports and every other subscription item.
			if (opts.Client == "windows" || opts.CompatibleSubscription) && (p.Protocol == "amneziawg" || p.Protocol == "ss2022") {
				if p.Protocol == "amneziawg" {
					name := clientcfg.ProfileDisplayName(p.Core)
					if name == "" {
						name = p.Core
					}
					link, err = clientcfg.WireGuardClientURI(name, link)
					if err != nil {
						errs = append(errs, fmt.Errorf("core %s: %w", p.Core, err))
						continue
					}
				} else {
					u, parseErr := url.Parse(link)
					if parseErr != nil || u.Scheme != "ss" {
						errs = append(errs, fmt.Errorf("core %s: invalid Shadowsocks profile", p.Core))
						continue
					}
					if name := clientcfg.ProfileDisplayName(p.Core); name != "" {
						u.Fragment = name
					}
					link = u.String()
				}
			} else if opts.CompatibleSubscription && (p.Protocol == "anytls" || p.Protocol == "tuic" || p.Protocol == "mieru-tcp" || p.Protocol == "mieru-udp") {
				// Standard URIs let older clients skip unsupported families without invalidating the old feed.
				u, parseErr := url.Parse(link)
				if parseErr != nil { errs = append(errs, fmt.Errorf("core %s: invalid standard profile",p.Core)); continue }
				name:=clientcfg.ProfileDisplayName(p.Core)
				if name=="" {name=clientcfg.ProfileDisplayName(p.Protocol)}
				if name!="" {u.Fragment=name;link=u.String()}
			} else if opts.Format != "standard" {
				link, err = clientcfg.GenerateCoreURI(p.Core, p.Link, cfg, clientcfg.URIOptions{Username: user.Username, DisplayName: clientcfg.ProfileDisplayName(p.Core)})
				if err != nil {
					errs = append(errs, fmt.Errorf("core %s: %w", p.Core, err))
					continue
				}
			}
			profiles = append(profiles, Profile{ID: p.Core, Protocol: p.Protocol, Link: link, Standard: p.Link, File: file, Rank: clientcfg.CoreRank(p.Protocol)})
		}
	}
	for _, t := range tunnels {
		protocol := t.Transport
		if protocol == config.TransportSoar && t.Backend == config.BackendDirect {
			protocol = "soar-direct"
		}
		if (user.SQLiteMeter != nil && user.QuotaGB > 0 && t.Transport != config.TransportVLESS) || !t.Enabled || !protocols.Allowed(protocol, user.MeteredOnly(), user.ExpiresAt != "") || (clientcfg.PlainSOCKS(&t) && !opts.All) {
			continue
		}
		if t.Transport == config.TransportVLESS && user.Username == "" {
			continue
		}
		for _, v := range clientcfg.Variants(&t) {
			vt := t
			vt.Backend, vt.Tag = v.Backend, v.Tag
			backend := cfg.GetBackend(v.Backend)
			noLogin := protocol == "soar-direct"
			if backend == nil && !noLogin && t.Transport != config.TransportVLESS {
				continue
			}
			modes := []string{""}
			if t.Transport == config.TransportDNSTT {
				modes = []string{clientcfg.ClientModeDNSTT, clientcfg.ClientModeNoizDNS}
			}
			for _, mode := range modes {
				id := vt.Tag
				if mode == clientcfg.ClientModeNoizDNS {
					id = strings.ReplaceAll(id, "dnstt", "noizdns")
					if id == vt.Tag {
						id = vt.Tag + "-2"
					} // preserve the old feed's second-mode ID for custom tags
				}
				link, err := clientcfg.GeneratePortableURI(&vt, backend, cfg, clientcfg.URIOptions{Username: user.Username, Password: user.Password, ClientMode: mode, DisplayName: clientcfg.ProfileDisplayName(id)}, opts.Format)
				if err != nil {
					errs = append(errs, fmt.Errorf("profile %s: %w", id, err))
					continue
				}
				standard := ""
				if t.Transport == config.TransportVLESS {
					standard, err = clientcfg.GenerateVLESSLink(&vt, cfg, user.Username, user.Password)
					if err != nil {
						errs = append(errs, fmt.Errorf("profile %s: %w", id, err))
						continue
					}
				}
				profiles = append(profiles, Profile{ID: id, Protocol: protocol, Link: link, Standard: standard, NoLogin: noLogin, Rank: clientcfg.TunnelRank(t.Transport)})
			}
		}
	}
	profiles = clientcfg.SortByRank(profiles, func(p Profile) int { return p.Rank })
	seen := map[string]bool{}
	for _, p := range profiles {
		if p.ID == "" || len(p.ID) > 200 || seen[p.ID] {
			return nil, fmt.Errorf("duplicate or invalid profile id %q", p.ID)
		}
		seen[p.ID] = true
	}
	return profiles, errors.Join(errs...)
}
