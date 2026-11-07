package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/system"
	"github.com/msVPNco/rovunet-panel/internal/warp"
)

const (
	maxLimitDays = 3650
	maxLimitGB   = 100000
)

// parseLimit reads a days/GB answer: blank, 0 or "free" mean unlimited.
func parseLimit(s string, max int) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "0", "free", "unlimited":
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > max {
		return 0, fmt.Errorf("enter a whole number from 1 to %d, or leave blank for unlimited", max)
	}
	return n, nil
}

// askUserLimits decides a new user's expiry and traffic limit. Days or GB
// passed as arguments (--days, --gb) skip the questions; otherwise the
// operator is asked "unlimited or limited?" and, if limited, for days and GB,
// each defaulting to unlimited.
func askUserLimits(daysArg, gbArg string) (days, gb int, err error) {
	if strings.TrimSpace(daysArg) != "" || strings.TrimSpace(gbArg) != "" {
		if days, err = parseLimit(daysArg, maxLimitDays); err != nil {
			return 0, 0, fmt.Errorf("days: %w", err)
		}
		if gb, err = parseLimit(gbArg, maxLimitGB); err != nil {
			return 0, 0, fmt.Errorf("gb: %w", err)
		}
		return days, gb, nil
	}
	mode, err := prompt.Select("Access for this user", []actions.SelectOption{
		{Value: "free", Label: "Unlimited (no expiry, no traffic limit)"},
		{Value: "limited", Label: "Limited (days and/or traffic)"},
	})
	if err != nil {
		return 0, 0, err
	}
	if mode != "limited" {
		return 0, 0, nil
	}
	for {
		raw, err := prompt.String("Days until expiry (blank = unlimited)", "")
		if err != nil {
			return 0, 0, err
		}
		if days, err = parseLimit(raw, maxLimitDays); err == nil {
			break
		}
		fmt.Println("  " + err.Error())
	}
	for {
		raw, err := prompt.String("Traffic limit in GB (blank = unlimited)", "")
		if err != nil {
			return 0, 0, err
		}
		if gb, err = parseLimit(raw, maxLimitGB); err == nil {
			break
		}
		fmt.Println("  " + err.Error())
	}
	return days, gb, nil
}

// newUser builds a user with the chosen limits, counted from now.
func newUser(username, password string, days, gb int, now time.Time) config.UserConfig {
	u := config.UserConfig{Username: username, Password: password, QuotaGB: gb, SubToken: config.NewSubToken()}
	if days > 0 {
		u.ExpiresAt = now.UTC().Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)
	}
	return u
}

// limitSummary describes a user's limits for lists and for the operator.
func limitSummary(u config.UserConfig) string {
	var parts []string
	if u.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, u.ExpiresAt); err == nil {
			parts = append(parts, "expires "+t.UTC().Format("2006-01-02 15:04 UTC"))
		}
	}
	if u.QuotaGB > 0 {
		parts = append(parts, fmt.Sprintf("%d GB", u.QuotaGB))
	}
	if len(parts) == 0 {
		return "unlimited"
	}
	return strings.Join(parts, ", ")
}

// removeUserFully removes a user from the system, the config and every
// service that holds credentials, and ends the user's open sessions.
func removeUserFully(cfg *config.Config, out actions.OutputWriter, username string) error {
	if err := system.RemoveSSHUser(username); err != nil {
		return fmt.Errorf("failed to remove SSH user: %w", err)
	}
	cfg.RemoveUser(username)
	if cfg.Warp.Enabled {
		proxy.RunAsUser = warp.SocksUser
	}
	_ = setupSOCKS(cfg)
	syncNaive(cfg, out)
	syncCores(cfg, out)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	if cfg.Warp.Enabled {
		if err := warp.RefreshRouting(cfg); err != nil {
			out.Warning("Failed to update WARP routing: " + err.Error())
		}
	}
	return nil
}

// ExpireUsers removes every user whose expiry has passed and returns their
// names. It stops being a no-op only when someone has expired.
func ExpireUsers(cfg *config.Config, out actions.OutputWriter, now time.Time) ([]string, error) {
	var gone []string
	for _, u := range append([]config.UserConfig(nil), cfg.Users...) {
		if !u.Expired(now) {
			continue
		}
		if err := removeUserFully(cfg, out, u.Username); err != nil {
			return gone, fmt.Errorf("%s: %w", u.Username, err)
		}
		gone = append(gone, u.Username)
	}
	return gone, nil
}

// HasExpiringUsers reports whether any user has an expiry date.
func HasExpiringUsers(cfg *config.Config) bool {
	for _, u := range cfg.Users {
		if u.ExpiresAt != "" {
			return true
		}
	}
	return false
}

const expiryService = `[Unit]
Description=Remove expired Rovunet users

[Service]
Type=oneshot
ExecStart=%s expire-users
`

const expiryTimer = `[Unit]
Description=Remove expired Rovunet users every hour

[Timer]
OnCalendar=hourly
RandomizedDelaySec=2min
Persistent=true

[Install]
WantedBy=timers.target
`

// ExpiryTimerName is the systemd timer that runs expire-users.
const ExpiryTimerName = "rovunet-user-expiry.timer"

// EnsureExpiryTimer installs the hourly timer when some user has an expiry.
// It is idempotent.
func EnsureExpiryTimer(cfg *config.Config) error {
	if !HasExpiringUsers(cfg) {
		return nil
	}
	self, err := config.SelfPath()
	if err != nil {
		return err
	}
	dir := "/etc/systemd/system"
	if err := os.WriteFile(filepath.Join(dir, "rovunet-user-expiry.service"), []byte(fmt.Sprintf(expiryService, self)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ExpiryTimerName), []byte(expiryTimer), 0644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", "--now", ExpiryTimerName}} {
		if out, err := exec.Command("systemctl", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
