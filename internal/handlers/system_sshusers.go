package handlers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
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

	if action == "" {
		var err error
		action, err = prompt.Select("Action", []actions.SelectOption{
			{Value: "add", Label: "Add user"},
			{Value: "bulk_add", Label: "Add multiple users (random creds)"},
			{Value: "edit_password", Label: "Edit user password"},
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
			if u.QuotaGB > 0 {
				line += fmt.Sprintf(" (used %s)", formatGB(UsedBytes(u, totals)))
			}
			out.Print(fmt.Sprintf("  %-20s %s", u.Username, line))
		}
		for _, a := range cfg.Archived {
			out.Print(fmt.Sprintf("  %-20s stopped: %s (restore with 'users --action renew')", a.Username, a.Reason))
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

			if err := system.AddSSHUser(uname, password); err != nil {
				out.Warning(fmt.Sprintf("failed to create %s: %v (skipping)", uname, err))
				continue
			}
			cfg.AddUser(newUser(uname, password, days, gb, time.Now()))
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
		if err := system.AddSSHUser(username, password); err != nil {
			return actions.NewError(actions.SystemUsers, "failed to update SSH password: "+err.Error(), err)
		}
		updated := *cfg.GetUser(username) // keeps the user's expiry and limit
		updated.Password = password
		updated.SubToken = config.NewSubToken() // the old link stops working
		cfg.AddUser(updated)

		if err := cfg.Save(); err != nil {
			return actions.NewError(actions.SystemUsers, "failed to save config", err)
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

	if err := system.AddSSHUser(username, password); err != nil {
		return fmt.Errorf("failed to create SSH user: %w", err)
	}

	cfg.AddUser(newUser(username, password, days, gb, time.Now()))

	// Save before SOCKS so the config file is always at least as current
	// as the SOCKS creds file; a failed save leaves SOCKS untouched.
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

	out.Success(fmt.Sprintf("User %q added (SSH + SOCKS): %s", username, limitSummary(*cfg.GetUser(username))))
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
		out.Print("")
	}
}

func printUserConfigs(cfg *config.Config, username, password string, out actions.OutputWriter) {
	ensureServerAddress(cfg, out, cfg.Tunnels)
	// Fastest first: Xray cores are slotted in among the tunnels by rank.
	limited := quotaLimited(cfg, username)
	flushCores := coreProfilePrinter(cfg, config.UserConfig{Username: username, Password: password}, out)
	if limited {
		flushCores = func(int) int { return 0 } // Xray cores are not measured
		out.Print("  # Traffic-limited user: only the VLESS config is given, because only its traffic is counted.")
	}
	hidden := 0
	for _, t := range clientcfg.SortByRank(cfg.Tunnels, tunnelRank) {
		flushCores(clientcfg.TunnelRank(t.Transport))
		if t.Transport == config.TransportVLESS {
			printVLESSLinks(out, cfg, &t, username, password)
			continue
		}
		if limited {
			continue
		}
		if skipPlainSOCKS(&t) {
			hidden++
			continue
		}
		// A NaiveProxy tunnel serves SOCKS and SSH clients alike: print both.
		for _, v := range naiveAwareVariants(&t) {
			vt := t
			vt.Backend = v.backend
			vt.Tag = v.tag
			printTunnelConfig(cfg, &vt, username, password, out)
		}
	}
	if !limited {
		flushCores(1 << 30)
	}
	noteHidden(out, hidden)
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

// printTunnelConfig prints one tunnel's config(s) for a user: its public key
// where it has one, then a rovunet:// link per client mode.
func printTunnelConfig(cfg *config.Config, t *config.TunnelConfig, username, password string, out actions.OutputWriter) {
	backend := cfg.GetBackend(t.Backend)
	// Soar's direct backend is built into the Soar server: there is no
	// backend entry, and its config needs none (it has no login).
	soarDirect := t.Transport == config.TransportSoar && t.Backend == config.BackendDirect
	if backend == nil && !soarDirect {
		return
	}

	// Show public key for DNSTT/VayDNS/Soar tunnels
	if t.Transport == config.TransportDNSTT && t.DNSTT != nil && t.DNSTT.PublicKey != "" {
		out.Print(fmt.Sprintf("  [%s] Public Key: %s", t.Tag, t.DNSTT.PublicKey))
	} else if t.Transport == config.TransportVayDNS && t.VayDNS != nil && t.VayDNS.PublicKey != "" {
		out.Print(fmt.Sprintf("  [%s] Public Key: %s", t.Tag, t.VayDNS.PublicKey))
	} else if t.Transport == config.TransportSoar && t.Soar != nil && t.Soar.PublicKey != "" {
		out.Print(fmt.Sprintf("  [%s] Public Key: %s", t.Tag, t.Soar.PublicKey))
	}

	modes := []string{""}
	if t.Transport == config.TransportDNSTT {
		modes = []string{clientcfg.ClientModeDNSTT, clientcfg.ClientModeNoizDNS}
	}

	for _, mode := range modes {
		opts := clientcfg.URIOptions{
			ClientMode: mode,
			Username:   username,
			Password:   password,
		}
		uri, err := clientcfg.GeneratePortableURI(t, backend, cfg, opts, "rovunet")
		if err != nil {
			continue
		}
		label := t.Tag
		if mode == clientcfg.ClientModeNoizDNS {
			label = strings.ReplaceAll(label, "dnstt", "noizdns")
		}
		if soarDirect {
			out.Print("  # Soar: no login, same for every user")
		}
		out.Print(fmt.Sprintf("  [%s] user=%s", label, username))
		out.Print(fmt.Sprintf("  %s", uri))
		out.Print("")
	}
}

// tunnelRank orders tunnels for output, fastest first.
func tunnelRank(t config.TunnelConfig) int { return clientcfg.TunnelRank(t.Transport) }

// coreProfilePrinter returns a function that prints the user's Xray core
// profiles whose rank is at most the given limit, in rank order, each once.
// Calling it with each tunnel's rank before the tunnel, and with a huge limit
// at the end, interleaves cores and tunnels from fastest to slowest.
func coreProfilePrinter(cfg *config.Config, user config.UserConfig, out actions.OutputWriter) func(limit int) int {
	profiles, err := coreinstall.Profiles(user)
	if err != nil {
		out.Warning("Xray cores: " + err.Error())
	}
	profiles = clientcfg.SortByRank(profiles, func(p coreinstall.Profile) int { return clientcfg.CoreRank(p.Protocol) })
	next := 0
	return func(limit int) int {
		printed := 0
		for next < len(profiles) && clientcfg.CoreRank(profiles[next].Protocol) <= limit {
			if printCoreProfile(cfg, user, profiles[next], out) {
				printed++
			}
			next++
		}
		return printed
	}
}

// printCoreProfiles prints all of a user's Xray core profiles, each wrapped so
// the app groups it with the user's tunnels, followed by the same profile for
// other apps. It returns how many it printed.
func printCoreProfiles(cfg *config.Config, user config.UserConfig, out actions.OutputWriter) int {
	return coreProfilePrinter(cfg, user, out)(1 << 30)
}

func printCoreProfile(cfg *config.Config, user config.UserConfig, p coreinstall.Profile, out actions.OutputWriter) bool {
	uri, err := clientcfg.GenerateCoreURI(p.Core, p.Link, cfg, clientcfg.URIOptions{Username: user.Username})
	if err != nil {
		out.Warning(fmt.Sprintf("Xray core %s: %v", p.Core, err))
		return false
	}
	out.Print(fmt.Sprintf("  [%s] %s, user=%s", p.Core, coreProtocolName(p.Protocol), user.Username))
	out.Print(fmt.Sprintf("  %s", uri))
	// The same profile for other clients: a one-line link, or the
	// WireGuard file to save and import.
	if p.Protocol == "wireguard" {
		out.Print("  # WireGuard file for other apps (save as rovunet.conf);")
		out.Print("  # its lines must start at the left edge, with no indentation:")
		for _, line := range strings.Split(strings.TrimRight(p.Link, "\n"), "\n") {
			out.Print(line)
		}
	} else {
		out.Print("  # Standard link for other apps:")
		out.Print("  " + p.Link)
	}
	out.Print("")
	return true
}

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
