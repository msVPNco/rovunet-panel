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
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/usage"
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
	u := config.UserConfig{Username: username, Password: password, QuotaGB: gb, SubToken: config.NewSubToken(), UsageBase: usage.Totals()[username],
		AccountID: config.NewID(), PeriodID: config.NewID()}
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
	// A traffic-limited user has no SSH account, so there may be nothing to remove.
	if err := revokeSSH(username); err != nil {
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

// overQuota reports whether a limited user has used up the traffic limit.
func overQuota(u config.UserConfig, totals map[string]int64) bool {
	return u.QuotaGB > 0 && totals[u.Username]-u.UsageBase >= u.QuotaBytes()
}

// UsedBytes is the traffic a user has used since creation or renewal.
func UsedBytes(u config.UserConfig, totals map[string]int64) int64 {
	if n := totals[u.Username] - u.UsageBase; n > 0 {
		return n
	}
	return 0
}

// retireUser removes a user from every service and keeps a record that
// "users renew" can bring back with the same password and link.
func retireUser(cfg *config.Config, out actions.OutputWriter, u config.UserConfig, reason string, now time.Time) error {
	cfg.Archived = append(cfg.Archived, config.ArchivedUser{UserConfig: u, Reason: reason, At: now.UTC().Format(time.RFC3339)})
	if err := removeUserFully(cfg, out, u.Username); err != nil {
		cfg.Archived = cfg.Archived[:len(cfg.Archived)-1]
		return err
	}
	return nil
}

// EnforceLimits retires every user whose expiry has passed or whose traffic
// limit is used up, and returns "name (reason)" for each.
func EnforceLimits(cfg *config.Config, out actions.OutputWriter, now time.Time) ([]string, error) {
	collectErr := coreinstall.CollectUsage()
	if collectErr != nil {
		out.Warning("Xray traffic: " + collectErr.Error())
	}
	// The status files say how fresh and how complete a user's count is.
	if err := usage.MarkCollected(now, collectErr == nil); err != nil {
		out.Warning("traffic counting: " + err.Error())
	}
	totals := usage.Totals()
	var gone []string
	for _, u := range append([]config.UserConfig(nil), cfg.Users...) {
		reason := ""
		switch {
		case u.Expired(now):
			reason = "expired"
		case overQuota(u, totals):
			reason = "traffic limit"
		default:
			continue
		}
		if err := retireUser(cfg, out, u, reason, now); err != nil {
			return gone, fmt.Errorf("%s: %w", u.Username, err)
		}
		gone = append(gone, fmt.Sprintf("%s (%s)", u.Username, reason))
	}
	retryRevocations(cfg, out)
	return gone, nil
}

// retryRevocations tries again whatever taking a user's access away could not
// finish earlier (a core that would not restart, a Caddyfile that was
// rejected), and removes the SSH account of any traffic-limited user who still
// has one. It runs on every limits check, so a removal never waits for the
// next user change.
func retryRevocations(cfg *config.Config, out actions.OutputWriter) {
	syncSSHAccess(cfg, out)
	syncNaive(cfg, out)
	if len(coreinstall.PendingRevocations()) > 0 {
		syncCores(cfg, out)
	}
}

// HasLimitedUsers reports whether any user has an expiry or a traffic limit.
func HasLimitedUsers(cfg *config.Config) bool {
	for _, u := range cfg.Users {
		if u.ExpiresAt != "" || u.QuotaGB > 0 {
			return true
		}
	}
	return false
}

// quotaLimited reports whether a user has a traffic limit.
func quotaLimited(cfg *config.Config, username string) bool {
	u := cfg.GetUser(username)
	return u != nil && u.QuotaGB > 0
}

const expiryService = `[Unit]
Description=Retire Rovunet users whose time or traffic is used up

[Service]
Type=oneshot
ExecStart=%s expire-users
`

const expiryTimer = `[Unit]
Description=Check Rovunet user time and traffic limits every 5 minutes

[Timer]
OnCalendar=*:0/5
RandomizedDelaySec=20
Persistent=true

[Install]
WantedBy=timers.target
`

// ExpiryTimerName is the systemd timer that runs expire-users.
const ExpiryTimerName = "rovunet-user-expiry.timer"

// EnsureExpiryTimer installs the 5-minute limits timer when some user has an
// expiry or a traffic limit. It is idempotent.
func EnsureExpiryTimer(cfg *config.Config) error {
	if !HasLimitedUsers(cfg) {
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

func formatGB(b int64) string { return fmt.Sprintf("%.2f GB", float64(b)/float64(1<<30)) }

// warnQuotaScope tells the operator what a traffic limit covers.
func warnQuotaScope(cfg *config.Config, out actions.OutputWriter) {
	out.Info("Traffic limit: only VLESS (WebSocket), REALITY and XHTTP traffic is counted, so this user is given only those configs, has no SSH account and no login on NaïveProxy, SOCKS5, Hysteria 2 or WireGuard. Counting updates every 15 seconds and the limit is checked every 5 minutes, so more than the limit can pass on a fast connection.")
	for _, t := range cfg.Tunnels {
		if t.Transport == config.TransportVLESS {
			return
		}
	}
	out.Warning("There is no VLESS tunnel yet, so this user would get no config. Add one with 'rovunet tunnel add'.")
}

// forgetArchived drops a stopped user's record for good.
func forgetArchived(cfg *config.Config, username string) bool {
	for i, a := range cfg.Archived {
		if a.Username == username {
			cfg.Archived = append(cfg.Archived[:i], cfg.Archived[i+1:]...)
			return true
		}
	}
	return false
}

// renewedUser is a stopped user brought back: the same password, link and
// account, a new period, and limits counted from now.
func renewedUser(arch config.ArchivedUser, days, gb int, now time.Time) config.UserConfig {
	u := newUser(arch.Username, arch.Password, days, gb, now) // a new PeriodID
	if arch.SubToken != "" {
		u.SubToken = arch.SubToken // the same link works again
	}
	if arch.AccountID != "" {
		u.AccountID = arch.AccountID
	}
	return u
}

// renewUser brings a stopped user back with the same password and link and
// new limits counted from now.
func renewUser(cfg *config.Config, out actions.OutputWriter, username, daysArg, gbArg string) error {
	if len(cfg.Archived) == 0 {
		return fmt.Errorf("no stopped users: nobody has expired or used up a traffic limit")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		opts := make([]actions.SelectOption, 0, len(cfg.Archived))
		for _, a := range cfg.Archived {
			opts = append(opts, actions.SelectOption{Value: a.Username, Label: fmt.Sprintf("%s (%s)", a.Username, a.Reason)})
		}
		var err error
		if username, err = prompt.Select("User to renew", opts); err != nil {
			return err
		}
	}
	var arch *config.ArchivedUser
	idx := -1
	for i := range cfg.Archived {
		if cfg.Archived[i].Username == username {
			arch, idx = &cfg.Archived[i], i
		}
	}
	if arch == nil {
		return fmt.Errorf("%q is not a stopped user", username)
	}
	if cfg.GetUser(username) != nil {
		return fmt.Errorf("user %q already exists", username)
	}
	days, gb, err := askUserLimits(daysArg, gbArg)
	if err != nil {
		return err
	}
	u := renewedUser(*arch, days, gb, time.Now())
	if err := grantSSH(u); err != nil {
		return fmt.Errorf("failed to create SSH user: %w", err)
	}
	cfg.AddUser(u)
	cfg.Archived = append(cfg.Archived[:idx], cfg.Archived[idx+1:]...)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	if cfg.Warp.Enabled {
		proxy.RunAsUser = warp.SocksUser
	}
	if err := setupSOCKS(cfg); err != nil {
		out.Warning("Failed to update SOCKS proxy auth: " + err.Error())
	}
	syncNaive(cfg, out)
	syncCores(cfg, out)
	if cfg.Warp.Enabled {
		if err := warp.RefreshRouting(cfg); err != nil {
			out.Warning("Failed to update WARP routing: " + err.Error())
		}
	}
	if days > 0 || gb > 0 {
		if err := EnsureExpiryTimer(cfg); err != nil {
			out.Warning("Could not set up the limits timer: " + err.Error())
		}
	}
	if gb > 0 {
		warnQuotaScope(cfg, out)
	}
	out.Success(fmt.Sprintf("User %q renewed: %s. The same password and subscription link work again.", username, limitSummary(u)))
	return nil
}
