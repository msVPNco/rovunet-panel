package handlers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

const donationDays = 30
const donationQuotaGB = 50

// The production path is unchanged; tests stop at a failed accounting boundary
// without installing services or creating operating-system accounts.
var finishNewUserAccounting = finishInstallationAccounting

// This private candidate contains deliberately shared access credentials.
// Export does not publish a catalog, sign it, or enable an app feature.
type donationCandidate struct {
	Version    int       `json:"version"`
	Kind       string    `json:"kind"`
	DonationID string    `json:"donation_id"`
	CreatedAt  string    `json:"created_at"`
	ExpiresAt  string    `json:"expires_at"`
	QuotaGB    int       `json:"quota_gb"`
	Items      []subItem `json:"items"`
}

func createDonationUser(cfg *config.Config, out actions.OutputWriter, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "gift-" + config.NewID()[:12]
	}
	if err := config.ValidateUsername(username); err != nil {
		return err
	}
	if err := coreinstall.ValidateName(username); err != nil {
		return err
	}
	for _, archived := range cfg.Archived {
		if archived.Username == username {
			return fmt.Errorf("user %q is archived; choose a new Free config username", username)
		}
	}
	if err := createUserInteractiveWithDonation(cfg, out, username, "", "", true); err != nil {
		return err
	}
	if err := writeDonationCandidate(cfg, username, out); err != nil {
		return fmt.Errorf("Free config account %q is saved; export pending (retry: rovunet users --action donation-export --username %s): %w", username, username, err)
	}
	return nil
}

func donationExpiry(u config.UserConfig, now time.Time) (time.Time, error) {
	if !u.Donation || u.QuotaGB != donationQuotaGB || !u.MeteredOnly() {
		return time.Time{}, fmt.Errorf("only marked 50 GiB Free config accounts can be exported")
	}
	expiry, err := time.Parse(time.RFC3339, u.ExpiresAt)
	if err != nil || !expiry.After(now) {
		return time.Time{}, fmt.Errorf("Free config has expired or has no valid expiry")
	}
	if expiry.After(now.Add(donationDays * 24 * time.Hour)) {
		return time.Time{}, fmt.Errorf("Free config expiry exceeds the 30-day policy")
	}
	if !validPublicationID(u.AccountID, u.PeriodID) {
		return time.Time{}, fmt.Errorf("Free config has no stable account and period identifiers")
	}
	return expiry, nil
}

func donationQuotaReady(cfg *config.Config, u config.UserConfig) error {
	if cfg.RecoverySealed || cfg.HasCarryHold(u.Username) || u.SQLiteMeter == nil {
		return fmt.Errorf("Free config requires completed server-side quota accounting")
	}
	amount, limit, err := boundAmount(cfg.ServerID, u)
	if err != nil {
		return fmt.Errorf("Free config quota accounting is unavailable or inactive: %w", err)
	}
	if limit != int64(donationQuotaGB)<<30 || amount.Confirmed >= limit || amount.Pending >= limit-amount.Confirmed {
		return fmt.Errorf("Free config quota is exhausted, fully reserved or differs from 50 GiB")
	}
	return nil
}

func buildDonationCandidate(u config.UserConfig, profiles []clientexport.Profile, now time.Time) (donationCandidate, error) {
	expiry, err := donationExpiry(u, now)
	if err != nil {
		return donationCandidate{}, err
	}
	candidate := donationCandidate{
		Version: 1, Kind: "rovunet-donation", DonationID: u.AccountID,
		CreatedAt: expiry.Add(-donationDays * 24 * time.Hour).UTC().Format(time.RFC3339),
		ExpiresAt: expiry.UTC().Format(time.RFC3339), QuotaGB: donationQuotaGB,
		Items: make([]subItem, 0, len(profiles)),
	}
	if len(profiles) == 0 || len(profiles) > 1000 {
		return donationCandidate{}, fmt.Errorf("no metered standard profiles available, or too many profiles")
	}
	seen := map[string]bool{}
	for _, p := range profiles {
		if p.ID == "" || len(p.ID) > 200 || seen[p.ID] || p.File || p.NoLogin {
			return donationCandidate{}, fmt.Errorf("invalid, duplicate or unaccounted public profile %q", p.ID)
		}
		seen[p.ID] = true
		parsed, err := url.Parse(p.Link)
		if err != nil || parsed.Hostname() == "" || parsed.Opaque != "" {
			return donationCandidate{}, fmt.Errorf("invalid public profile %q", p.ID)
		}
		query, err := url.ParseQuery(parsed.RawQuery)
		if err != nil {
			return donationCandidate{}, fmt.Errorf("invalid public profile query %q", p.ID)
		}
		for key, values := range query {
			switch strings.ToLower(key) {
			case "allowinsecure", "insecure", "skip-cert-verify", "skipcertverify":
				for _, value := range values {
					if value != "0" && !strings.EqualFold(value, "false") {
						return donationCandidate{}, fmt.Errorf("insecure public profile %q", p.ID)
					}
				}
			}
		}
		switch strings.ToLower(parsed.Scheme) {
		case "vless":
			security := query["security"]
			if len(security) != 1 || (security[0] != "tls" && security[0] != "reality") {
				return donationCandidate{}, fmt.Errorf("insecure VLESS profile %q", p.ID)
			}
		case "ss", "trojan", "hysteria2", "hy2":
			// Eligibility is established by clientexport and the bound ledger;
			// accepting a URI scheme alone never grants an unmetered protocol.
		default:
			return donationCandidate{}, fmt.Errorf("no compatible public link for %q (%s)", p.ID, parsed.Scheme)
		}
		candidate.Items = append(candidate.Items, subItem{ID: p.ID, Link: p.Link})
	}
	return candidate, nil
}

func writeDonationCandidate(cfg *config.Config, username string, out actions.OutputWriter) error {
	if err := coreinstall.ValidateName(username); err != nil {
		return err
	}
	if err := cfg.RefreshIfChanged(); err != nil {
		return err
	}
	user := cfg.GetUser(username)
	if user == nil {
		return fmt.Errorf("Free config account %q not found", username)
	}
	u := *user
	if _, err := donationExpiry(u, time.Now()); err != nil {
		return err
	}
	if err := donationQuotaReady(cfg, u); err != nil {
		return err
	}
	profiles, err := clientexport.Build(cfg, u, cfg.Tunnels, clientexport.Options{
		Format: "standard", Client: "android", StatsWorking: statsWorking,
	})
	if err != nil {
		return fmt.Errorf("could not render eligible Free config profiles: %w", err)
	}
	candidate, err := buildDonationCandidate(u, profiles, time.Now())
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(filepath.Dir(cfg.Path()), "donations")
	path := filepath.Join(dir, username+".json")
	err = cfg.WithCurrentFile(func() error {
		if _, err := donationExpiry(u, time.Now()); err != nil {
			return err
		}
		if err := donationQuotaReady(cfg, u); err != nil {
			return err
		}
		if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || info.Mode().Perm()&0077 != 0 || !ok || int(owner.Uid) != os.Geteuid() {
			return fmt.Errorf("Free config export directory must be private and owned by the panel operator")
		}
		return safefile.Write(path, append(data, '\n'), 0600)
	})
	if err != nil {
		return err
	}
	out.Success(fmt.Sprintf("Free config candidate (%d links; 50 GiB shared / 30 days): %s", len(candidate.Items), path))
	out.Info("Review this private candidate before catalog signing or publication. The export does not publish credentials or enable the app's Free config feature.")
	return nil
}
