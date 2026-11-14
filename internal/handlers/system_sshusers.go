package handlers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/system"
	"github.com/msVPNco/rovunet-panel/internal/usage"
	"github.com/msVPNco/rovunet-panel/internal/warp"
)

// bulkAddMax caps the number of users per bulk_add invocation. A single call
// only triggers one SOCKS reload + one WARP rule sync regardless of N, so
// the cap is about bounding useradd/chpasswd time, not cascading restarts.
const bulkAddMax = 500

func handleSystemUsers(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	out := ctx.Output
	action := ctx.GetArg("action")
	username := ctx.GetArg("username")

	menuSelected := action == ""
	if action == "" {
		var err error
		action, err = prompt.Select("Action", []actions.SelectOption{
			{Value: "add", Label: "Add user"},
			{Value: "bulk_add", Label: "Add multiple users (random creds)"},
			{Value: "edit_password", Label: "Edit user password"},
			{Value: "rotate_link", Label: "Replace subscription link (keep connection credentials)"},
			{Value: "renew", Label: "Renew a stopped user"},
			{Value: "remove", Label: "Remove user"},
			{Value: "list", Label: "List users & configs"},
			{Value: "configs", Label: "Show all configs (every tunnel and user)"},
		})
		if err != nil {
			return err
		}
	}

	switch action {
	case "configs":
		out.Print("")
		if printClientConfigs(cfg, out, cfg.Tunnels) == 0 {
			out.Info("No tunnels with app configs. Run 'rovunet tunnel add' to create one.")
		}

	case "list":
		if len(cfg.Users) == 0 {
			out.Info("No users configured. Run 'rovunet users add' to create one, or 'rovunet users configs' for configs that need no login.")
			return nil
		}

		// Show user list
		out.Print("")
		totals := usage.Totals()
		for _, u := range cfg.Users {
			line := limitSummary(u)
			if u.SQLiteMeter != nil {
				a, _, e := boundAmount(cfg.ServerID, u)
				if e != nil {
					line += " (usage unavailable)"
				} else {
					line += fmt.Sprintf(" (used %s)", formatGB(a.Confirmed))
				}
			} else if u.QuotaGB > 0 {
				line += fmt.Sprintf(" (used %s)", formatGB(UsedBytes(u, totals)))
			}
			out.Print(fmt.Sprintf("  %-20s %s", u.Username, line))
		}
		for _, a := range cfg.Archived {
			out.Print(fmt.Sprintf("  %-20s stopped: %s (restore with 'users --action renew')", a.Username, a.Reason))
		}
		for _, note := range pendingRevocationNotes() {
			out.Warning(note)
		}
		out.Print("")

		// Let user pick one to see configs
		userOpts := make([]actions.SelectOption, 0, len(cfg.Users))
		for _, u := range cfg.Users {
			userOpts = append(userOpts, actions.SelectOption{Value: u.Username, Label: u.Username})
		}
		selectedUser, err := prompt.Select("Show configs for user", userOpts)
		if err != nil {
			return err
		}

		user := cfg.GetUser(selectedUser)
		if user == nil {
			return actions.NewError(actions.SystemUsers, "user not found", nil)
		}

		out.Print("")
		out.Print(fmt.Sprintf("  User: %s", user.Username))
		out.Print(fmt.Sprintf("  Password: %s", user.Password))
		out.Print("")

		if len(cfg.Tunnels) == 0 {
			out.Print("  No tunnels configured")
		} else {
			showUserConfigs(cfg, user.Username, user.Password, out)
		}

	case "add":
		if err := createUserInteractive(cfg, out, username, ctx.GetArg("days"), ctx.GetArg("gb")); err != nil {
			return actions.NewError(actions.SystemUsers, err.Error(), nil)
		}

	case "renew":
		if err := renewUser(cfg, out, username, ctx.GetArg("days"), ctx.GetArg("gb")); err != nil {
			return actions.NewError(actions.SystemUsers, err.Error(), nil)
		}

	case "bulk_add":
		count, err := resolveBulkCount(ctx)
		if err != nil {
			return actions.NewError(actions.SystemUsers, err.Error(), nil)
		}

		prefix := strings.TrimSpace(ctx.GetArg("prefix"))
		if prefix == "" {
			prefix, err = prompt.String("Username prefix", "user")
			if err != nil {
				return err
			}
			prefix = strings.TrimSpace(prefix)
			if prefix == "" {
				prefix = "user"
			}
		}
		if err := config.ValidateTagName(prefix); err != nil {
			// Usernames share the tag-name character set (lowercase letters,
			// numbers, hyphens) so they round-trip safely through useradd and
			// sshd_config. Reject anything else upfront.
			return actions.NewError(actions.SystemUsers, "prefix "+err.Error(), nil)
		}

		days, gb, err := askUserLimits(ctx.GetArg("days"), ctx.GetArg("gb"))
		if err != nil {
			return actions.NewError(actions.SystemUsers, err.Error(), nil)
		}

		type newCred struct{ Username, Password string }
		created := make([]newCred, 0, count)

		next := nextUsernameIndex(cfg, prefix)
		for i := 0; i < count; i++ {
			uname := fmt.Sprintf("%s%d", prefix, next)
			next++
			if cfg.GetUser(uname) != nil {
				i--
				continue
			}
			password := system.GeneratePassword(16)

			nu := newUser(uname, password, days, gb, time.Now())
			if err := rememberNewPeriod(cfg, nu); err != nil {
				return err
			}
			if err := grantSSH(nu); err != nil {
				out.Warning(fmt.Sprintf("failed to create %s: %v (skipping)", uname, err))
				continue
			}
			cfg.AddUser(nu)
			created = append(created, newCred{uname, password})
		}

		if len(created) == 0 {
			return actions.NewError(actions.SystemUsers, "no users were created", nil)
		}

		// Save first — the config file is the source of truth. If this
		// fails we bail without touching SOCKS, so a retry just rewrites
		// the same state instead of leaving derived state ahead of disk.
		if err := cfg.Save(); err != nil {
			return actions.NewError(actions.SystemUsers, "failed to save config", err)
		}

		fresh := map[string]bool{}
		for _, row := range created {
			fresh[cfg.GetUser(row.Username).AccountID] = true
		}
		if err := finishInstallationAccounting(cfg, fresh); err != nil {
			return err
		}

		// One SOCKS reload for the whole batch. With the hot-reload path this
		// is a single SIGHUP to the SOCKS service, not N restarts.
		if cfg.Warp.Enabled {
			proxy.RunAsUser = warp.SocksUser
		}
		if err := setupSOCKS(cfg); err != nil {
			out.Warning("Failed to update SOCKS proxy auth: " + err.Error())
		}
		syncNaive(cfg, out)
		syncCores(cfg, out)

		// One WARP live rule sync for the whole batch.
		if cfg.Warp.Enabled {
			if err := warp.RefreshRouting(cfg); err != nil {
				out.Warning("Failed to update WARP routing: " + err.Error())
			}
		}

		if days > 0 {
			if err := EnsureExpiryTimer(cfg); err != nil {
				out.Warning("Could not set up the expiry timer: " + err.Error() + " (the users will not be removed automatically)")
			}
		}
		if gb > 0 {
			warnQuotaScope(cfg, out)
		}
		out.Success(fmt.Sprintf("Created %d users (%s)", len(created), limitSummary(newUser("", "", days, gb, time.Now()))))
		out.Print("")
		for _, c := range created {
			out.Print(fmt.Sprintf("  %s : %s", c.Username, c.Password))
		}
		out.Print("")

		if len(cfg.Tunnels) > 0 {
			show, _ := prompt.Confirm("Show tunnel configs for each user?")
			if show {
				for _, c := range created {
					out.Print("")
					out.Print(fmt.Sprintf("── %s ──", c.Username))
					showUserConfigs(cfg, c.Username, c.Password, out)
				}
			}
		}

	case "rotate_link":
		if username == "" {
			var err error
			username, err = prompt.String("Username", "")
			if err != nil {
				return err
			}
		}
		id := ctx.GetArg("operation-id")
		if menuSelected && id == "" {
			choice, e := prompt.Select("Replacement request", []actions.SelectOption{{Value: "retry", Label: "Repeat default request (returns the same link)"}, {Value: "new", Label: "Start a new replacement"}, {Value: "id", Label: "Retry a saved request ID"}})
			if e != nil {
				return e
			}
			if choice == "new" {
				id = config.NewID()
				out.Info("Save this request ID to retry: " + id)
			} else if choice == "id" {
				id, e = prompt.String("Saved request ID", "")
				if e != nil {
					return e
				}
				if !tokenOK(id) {
					return fmt.Errorf("invalid saved request ID")
				}
			}
		}
		if err := RotateSubscriptionOperation(cfg, strings.TrimSpace(username), id); err != nil {
			return actions.NewError(actions.SystemUsers, "subscription link replacement did not complete", err)
		}
		u := cfg.GetUser(strings.TrimSpace(username))
		if u == nil {
			return config.ErrConcurrentChange
		}
		out.Success("Subscription link replaced; connection credentials and account identity were kept")
		out.Info("Previously imported configs retain their server access. This action revokes only the old download link.")
		out.Print("Android: " + SubscriptionURL(cfg, *u))
		out.Print("Windows: " + WindowsSubscriptionURL(cfg, *u))

	case "edit_password":
		if username == "" {
			if len(cfg.Users) == 0 {
				return actions.NewError(actions.SystemUsers, "no users configured", nil)
			}
			userOpts := make([]actions.SelectOption, 0, len(cfg.Users))
			for _, u := range cfg.Users {
				userOpts = append(userOpts, actions.SelectOption{Value: u.Username, Label: u.Username})
			}
			selected, err := prompt.Select("User to edit", userOpts)
			if err != nil {
				return err
			}
			username = selected
		}
		username = strings.TrimSpace(username)
		if username == "" {
			return actions.NewError(actions.SystemUsers, "username is required", nil)
		}
		if cfg.GetUser(username) == nil {
			return actions.NewError(actions.SystemUsers, fmt.Sprintf("user %q not found", username), nil)
		}

		password := strings.TrimSpace(ctx.GetArg("password"))
		if password == "" {
			var err error
			password, err = prompt.String("New password (leave blank to generate)", "")
			if err != nil {
				return err
			}
		}
		if password == "" {
			password = system.GeneratePassword(16)
			out.Info(fmt.Sprintf("Generated password: %s", password))
		} else if err := config.ValidatePassword(password); err != nil {
			return actions.NewError(actions.SystemUsers, err.Error(), nil)
		}

		// AddSSHUser is upsert-style: existing OS user keeps their entry and
		// just gets chpasswd'd. Same for cfg.AddUser, which updates in place.
		updated := *cfg.GetUser(username) // keeps the user's expiry and limit
		updated.Password = password
		updated.SubToken = config.NewSubToken() // the old link stops working
		cfg.AddUser(updated)

		if err := cfg.Save(); err != nil {
			return actions.NewError(actions.SystemUsers, "failed to save config", err)
		}
		// Commit the source of truth before changing OS credentials. A stale
		// concurrent command must not change SSH while its config save fails.
		if err := grantSSH(updated); err != nil {
			return actions.NewError(actions.SystemUsers, "password saved; SSH synchronization is pending; retry the password action", err)
		}
		if !updated.MeteredOnly() {
			if err := system.TerminateSSHUserSessions(username); err != nil {
				out.Warning("SSH password changed, but ending existing sessions was not confirmed")
			}
		}

		if cfg.Warp.Enabled {
			proxy.RunAsUser = warp.SocksUser
		}
		if err := setupSOCKS(cfg); err != nil {
			out.Warning("Failed to update SOCKS proxy auth: " + err.Error())
		}
		syncNaive(cfg, out)
		syncCores(cfg, out)

		out.Success(fmt.Sprintf("Password updated for %q", username))

		if len(cfg.Tunnels) > 0 {
			show, _ := prompt.Confirm("Show updated tunnel configs?")
			if show {
				out.Print("")
				showUserConfigs(cfg, username, password, out)
			}
		}

	case "remove":
		if username == "" {
			var err error
			username, err = prompt.String("Username", "")
			if err != nil {
				return err
			}
		}
		username = strings.TrimSpace(username)
		if username == "" {
			return actions.NewError(actions.SystemUsers, "username is required", nil)
		}

		if cfg.GetUser(username) == nil {
			if forgetArchived(cfg, username) {
				if err := cfg.Save(); err != nil {
					return actions.NewError(actions.SystemUsers, "failed to save config", err)
				}
				out.Success(fmt.Sprintf("Stopped user %q forgotten", username))
				return nil
			}
		}
		if err := removeUserFully(cfg, out, username); err != nil {
			return actions.NewError(actions.SystemUsers, err.Error(), err)
		}

		out.Success(fmt.Sprintf("User %q removed", username))
	}

	return nil
}

// createUserInteractive walks the operator through adding one user: prompts
// for name and password (generating a random one if blank), creates the
// system user, triggers the live SOCKS credential reload and WARP rule sync,
// and prints the resulting tunnel configs. Pass providedUsername to skip the
// first prompt (e.g. when the name came in as a CLI arg); pass "" to prompt.
// Used from both the `users add` action and the post-`tunnel add` offer.
func createUserInteractive(cfg *config.Config, out actions.OutputWriter, providedUsername, daysArg, gbArg string) error {
	username := providedUsername
	if username == "" {
		var err error
		username, err = prompt.String("Username", "")
		if err != nil {
			return err
		}
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if cfg.GetUser(username) != nil {
		return fmt.Errorf("user %q already exists", username)
	}

	password, err := prompt.String("Password (leave blank to generate)", "")
	if err != nil {
		return err
	}
	if password == "" {
		password = system.GeneratePassword(16)
		out.Info(fmt.Sprintf("Generated password: %s", password))
	} else if err := config.ValidatePassword(password); err != nil {
		return err
	}

	days, gb, err := askUserLimits(daysArg, gbArg)
	if err != nil {
		return err
	}

	nu := newUser(username, password, days, gb, time.Now())
	if err := rememberNewPeriod(cfg, nu); err != nil {
		return err
	}
	if err := grantSSH(nu); err != nil {
		return fmt.Errorf("failed to create SSH user: %w", err)
	}

	cfg.AddUser(nu)

	// Save before SOCKS so the config file is always at least as current
	// as the SOCKS creds file; a failed save leaves SOCKS untouched.
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if err := finishInstallationAccounting(cfg, map[string]bool{nu.AccountID: true}); err != nil {
		return err
	}
	if cfg.Warp.Enabled {
		proxy.RunAsUser = warp.SocksUser
	}
	if err := setupSOCKS(cfg); err != nil {
		out.Warning("Failed to update SOCKS proxy auth: " + err.Error())
	}
	syncNaive(cfg, out)
	if err := coreinstall.Sync(cfg.Users); err != nil {
		return fmt.Errorf("account saved; core synchronization pending (run core-sync-users): %w", err)
	}
	if err := WriteSubscriptions(cfg, out); err != nil {
		return fmt.Errorf("account saved; subscription publication pending (run subscriptions-refresh): %w", err)
	}

	if cfg.Warp.Enabled {
		if err := warp.RefreshRouting(cfg); err != nil {
			out.Warning("Failed to update WARP routing: " + err.Error())
		}
	}

	u := *cfg.GetUser(username)
	out.Success(fmt.Sprintf("User %q added (%s): %s", username, userAccessSummary(u), limitSummary(u)))
	if days > 0 {
		if err := EnsureExpiryTimer(cfg); err != nil {
			out.Warning("Could not set up the expiry timer: " + err.Error() + " (the user will not be removed automatically)")
		}
	}
	if gb > 0 {
		warnQuotaScope(cfg, out)
	}

	if len(cfg.Tunnels) > 0 {
		out.Print("")
		out.Info("Configs for this user:")
		showUserConfigs(cfg, username, password, out)
	}
	return nil
}

// resolveBulkCount gets the bulk_add count either from the CLI arg or via
// interactive prompt, and validates it.
func resolveBulkCount(ctx *actions.Context) (int, error) {
	raw := strings.TrimSpace(ctx.GetArg("count"))
	if raw == "" {
		var err error
		raw, err = prompt.String("How many users to create", "10")
		if err != nil {
			return 0, err
		}
		raw = strings.TrimSpace(raw)
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("count must be a positive integer")
	}
	if n > bulkAddMax {
		return 0, fmt.Errorf("count cannot exceed %d", bulkAddMax)
	}
	return n, nil
}

// nextUsernameIndex finds the lowest 1-based index N such that `<prefix>N`
// through `<prefix>(N+count-1)` do not already exist. Callers still check
// GetUser in the allocation loop in case of concurrent edits.
func nextUsernameIndex(cfg *config.Config, prefix string) int {
	highest := 0
	for _, u := range cfg.Users {
		if !strings.HasPrefix(u.Username, prefix) {
			continue
		}
		suffix := u.Username[len(prefix):]
		n, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if n > highest {
			highest = n
		}
	}
	return highest + 1
}

// showUserConfigs prints a user's configs, then the subscription link that
// delivers the same configs to the app.
func showUserConfigs(cfg *config.Config, username, password string, out actions.OutputWriter) {
	printUserConfigs(cfg, username, password, out)
	if u := cfg.GetUser(username); u != nil {
		printSubscriptionLink(cfg, *u, out)
	}
}

// printSubscriptionLink prints a user's subscription link, if the user has
// one. Users made before links existed get theirs written first.
func printSubscriptionLink(cfg *config.Config, u config.UserConfig, out actions.OutputWriter) {
	if u.Username == "" {
		return
	}
	if u.SubToken == "" {
		_ = WriteSubscriptions(cfg, discardOutput{}) // mints missing tokens
		if cu := cfg.GetUser(u.Username); cu != nil {
			u = *cu
		}
	}
	if link := SubscriptionURL(cfg, u); link != "" {
		out.Print("  Subscription link (all configs of " + u.Username + ", for the app's Subscriptions menu):")
		out.Print("  " + link)
		out.Print("  Windows 0.1.0 Beta subscription (excludes unsupported AWG / SS2022):")
		out.Print("  " + WindowsSubscriptionURL(cfg, u))
		out.Print("")
	}
}

func printUserConfigs(cfg *config.Config, username, password string, out actions.OutputWriter) {
	ensureServerAddress(cfg, out, cfg.Tunnels)
	u := config.UserConfig{Username: username, Password: password}
	if stored := cfg.GetUser(username); stored != nil {
		u = *stored
	}
	printStructuredProfiles(cfg, u, cfg.Tunnels, out, false)
}

func printStructuredProfiles(cfg *config.Config, u config.UserConfig, tunnels []config.TunnelConfig, out actions.OutputWriter, summary bool) int {
	profiles, err := clientexport.Build(cfg, u, tunnels, clientexport.Options{All: showAllConfigs(), StatsWorking: statsWorking})
	if err != nil {
		out.Warning("Client profiles: " + err.Error())
	}
	for _, p := range profiles {
		label := "user=" + u.Username
		if summary {
			label = u.Username
			if label == "" {
				label = "(no auth)"
			}
			if p.NoLogin {
				label = u.Username + ", no login needed"
				if u.Username == "" {
					label = "(no login)"
				}
			}
		}
		out.Print(fmt.Sprintf("  [%s] %s", p.ID, label))
		out.Print("  " + p.Link)
		if p.Standard != "" && p.Standard != p.Link {
			if p.File {
				out.Print("  # Private client file (save without indentation):")
				for _, line := range strings.Split(strings.TrimRight(p.Standard, "\n"), "\n") {
					out.Print(line)
				}
			} else {
				out.Print("  # Standard link for other apps:")
				out.Print("  " + p.Standard)
			}
		}
		out.Print("")
	}
	hidden := 0
	for _, t := range tunnels {
		if t.Enabled && skipPlainSOCKS(&t) && !u.MeteredOnly() {
			hidden++
		}
	}
	noteHidden(out, hidden)
	return len(profiles)
}

// showAllConfigs lists plain SOCKS5 tunnels too (ROVUNET_ALL_CONFIGS=1).
func showAllConfigs() bool { return os.Getenv("ROVUNET_ALL_CONFIGS") == "1" }

// skipPlainSOCKS reports whether the default output leaves this tunnel out.
func skipPlainSOCKS(t *config.TunnelConfig) bool {
	return clientcfg.PlainSOCKS(t) && !showAllConfigs()
}

func noteHidden(out actions.OutputWriter, n int) {
	if n > 0 {
		out.Print("  " + fmt.Sprintf(clientcfg.HiddenNote, n))
	}
}

// statsWorking is the shared export's statistics readiness check.
var statsWorking = coreinstall.StatsWorking

// coreProtocolName is a core protocol's standard name.
func coreProtocolName(protocol string) string {
	switch protocol {
	case "reality":
		return "VLESS + REALITY"
	case "xhttp":
		return "VLESS + XHTTP"
	case "hysteria2":
		return "Hysteria 2"
	case "wireguard":
		return "WireGuard"
	case "amneziawg":
		return "AmneziaWG (userspace)"
	case "ss2022":
		return "Shadowsocks 2022"
	case "trojan":
		return "Trojan TLS"
	case "vlessenc":
		return "VLESS Encryption (ML-KEM-768)"
	}
	return protocol
}

// syncCores brings the Xray cores in line with the panel's users; call it
// wherever users change, next to syncNaive.
func syncCores(cfg *config.Config, out interface{ Warning(string) }) {
	if err := coreinstall.Sync(cfg.Users); err != nil {
		out.Warning("Failed to update Xray cores: " + err.Error())
	}
}
