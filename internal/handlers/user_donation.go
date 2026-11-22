package handlers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
)

const donationDays = 30
const donationQuotaGB = 50

// A donation candidate deliberately contains public-use access credentials.
// It must be reviewed and signed by the separate catalog publisher before
// being made available to apps. The panel never receives a GitHub write token.
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
	if username == "" {
		username = "gift-" + config.NewID()[:12]
	}
	// Reuse the existing metered user onboarding and server-side enforcement.
	// No protocol that bypasses traffic accounting is granted to this account.
	if err := createUserInteractive(cfg, out, username, "30", "50"); err != nil {
		return err
	}
	u := cfg.GetUser(username)
	if u == nil {
		return fmt.Errorf("donation account was not saved")
	}
	u.Donation = true
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("donation account created but marking it for export failed: %w", err)
	}
	if err := writeDonationCandidate(cfg, username, out); err != nil {
		out.Warning("Donation account is saved; candidate export pending: " + err.Error())
		out.Info("After fixing eligible metered protocols, retry: rovunet users --action donation-export --username " + username)
	}
	return nil
}

func buildDonationCandidate(u config.UserConfig, profiles []clientexport.Profile, now time.Time) (donationCandidate, error) {
	var candidate donationCandidate
	if !u.Donation || u.QuotaGB != donationQuotaGB || !u.MeteredOnly() {
		return candidate, fmt.Errorf("only marked 50 GiB donation accounts can be exported")
	}
	expiry, err := time.Parse(time.RFC3339, u.ExpiresAt)
	if err != nil || !expiry.After(now) {
		return candidate, fmt.Errorf("donation has expired or has no valid expiry")
	}
	createdAt := expiry.Add(-donationDays * 24 * time.Hour)
	if createdAt.After(now.Add(time.Minute)) {
		return candidate, fmt.Errorf("donation expiry does not match the 30-day policy")
	}
	if u.AccountID == "" {
		return candidate, fmt.Errorf("donation has no stable account identifier")
	}
	candidate = donationCandidate{
		Version: 1, Kind: "rovunet-donation",
		DonationID: u.AccountID,
		CreatedAt:  createdAt.UTC().Format(time.RFC3339),
		ExpiresAt:  expiry.UTC().Format(time.RFC3339),
		QuotaGB:    donationQuotaGB,
		Items:      make([]subItem, 0, len(profiles)),
	}
	for _, p := range profiles {
		parsed, err := url.Parse(p.Link)
		if err != nil || parsed == nil {
			return donationCandidate{}, fmt.Errorf("invalid public profile %q", p.ID)
		}
		switch strings.ToLower(parsed.Scheme) {
		case "vless":
			if parsed.Query().Get("security") == "none" ||
				parsed.Query().Get("allowInsecure") == "1" {
				return donationCandidate{}, fmt.Errorf("insecure VLESS profile %q", p.ID)
			}
		case "ss", "trojan", "hysteria2", "hy2":
			// Only server-metered, eligible standard URLs from clientexport.
		default:
			return donationCandidate{}, fmt.Errorf("no compatible public link for %q (%s)", p.ID, parsed.Scheme)
		}
		candidate.Items = append(candidate.Items, subItem{ID: p.ID, Link: p.Link})
	}
	if len(candidate.Items) == 0 {
		return donationCandidate{}, fmt.Errorf("no metered standard profiles installed; install/repair eligible cores first")
	}
	return candidate, nil
}

func writeDonationCandidate(cfg *config.Config, username string, out actions.OutputWriter) error {
	if err := coreinstall.ValidateName(username); err != nil {
		return err
	}
	u := cfg.GetUser(username)
	if u == nil {
		return fmt.Errorf("donation account %q not found", username)
	}
	// Standard URIs are imported by public-feed readers; private native
	// subscriptions and SSH credentials must never be sent to the catalog.
	profiles, err := clientexport.Build(cfg, *u, cfg.Tunnels, clientexport.Options{
		Format: "standard", Client: "android", StatsWorking: statsWorking,
	})
	if err != nil {
		return fmt.Errorf("could not render eligible donation profiles: %w", err)
	}
	candidate, err := buildDonationCandidate(*u, profiles, time.Now())
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(config.DefaultConfigDir, "donations")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, username+".json")
	if err := safefile.Write(path, append(data, '\n'), 0600); err != nil {
		return err
	}
	out.Success(fmt.Sprintf("Donation candidate (%d public-use links; 50 GiB / 30 days): %s", len(candidate.Items), path))
	out.Warning("The candidate contains live connection credentials. Publish only after explicit review and catalog signing; never commit the panel config or a private user's feed.")
	return nil
}
