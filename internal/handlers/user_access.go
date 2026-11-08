package handlers

import (
	"fmt"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/proxy"
	"github.com/msVPNco/rovunet-panel/internal/system"
)

// A user with a traffic limit may use only the paths whose traffic is counted
// (VLESS WebSocket, REALITY, XHTTP): config.UserConfig.MeteredOnly is the one
// rule. Every service that holds a per-user credential follows it, so that
// hiding a config from the output is never the only thing keeping the user off
// an uncounted path:
//
//   - Xray cores:   coreinstall.usersFor leaves the user out of Hysteria 2 and WireGuard
//   - NaïveProxy:   transport leaves the user out of basic_auth
//   - SOCKS5:       setupSOCKS leaves the user out of the credentials file
//   - SSH:          grantSSH / revokeSSH below: no login account
//
// The user's record, limit and subscription stay either way.

// Seams for tests.
var (
	sshAccountExists = func(name string) bool { return system.UserExists(name) && system.IsSSHUser(name) }
	addSSHAccount    = system.AddSSHUser
	removeSSHAccount = system.RemoveSSHUser
)

// grantSSH gives u their SSH login account, or, for a traffic-limited user,
// makes sure they have none.
func grantSSH(u config.UserConfig) error {
	if u.MeteredOnly() {
		return revokeSSH(u.Username)
	}
	return addSSHAccount(u.Username, u.Password)
}

// revokeSSH removes a user's Rovunet SSH account and ends their open sessions.
// It does nothing when there is none, and never touches an account that
// Rovunet did not create.
func revokeSSH(username string) error {
	if !sshAccountExists(username) {
		return nil
	}
	return removeSSHAccount(username)
}

// syncSSHAccess takes the SSH account away from every traffic-limited user who
// still has one, e.g. a user limited by an older version of the panel.
func syncSSHAccess(cfg *config.Config, out interface{ Warning(string) }) {
	for _, u := range cfg.Users {
		if !u.MeteredOnly() {
			continue
		}
		if err := revokeSSH(u.Username); err != nil {
			out.Warning(fmt.Sprintf("Could not remove the SSH account of limited user %q: %v", u.Username, err))
		}
	}
}

// socksLockedUser is the placeholder credential that keeps the loopback SOCKS5
// proxy asking for a login when every user is traffic-limited.
const socksLockedUser = "rovunet-locked"

// socksCredentials is the set of logins the SOCKS5 proxy accepts: the users
// allowed on uncounted services. When panel users exist but all of them are
// traffic-limited, a placeholder nobody knows keeps the proxy asking for a
// login; an empty list would turn the loopback proxy into an open one.
func socksCredentials(cfg *config.Config) []config.UserConfig {
	users := config.UnmeteredUsers(cfg.Users)
	if len(cfg.Users) > 0 && len(users) == 0 {
		return []config.UserConfig{{Username: socksLockedUser, Password: system.GeneratePassword(32)}}
	}
	return users
}

// setupSOCKS (re)creates the SOCKS5 proxy for the users allowed on uncounted
// services, on all interfaces when a direct-socks5 tunnel exposes it.
func setupSOCKS(cfg *config.Config) error {
	users := socksCredentials(cfg)
	for _, t := range cfg.Tunnels {
		if t.Transport == config.TransportSOCKS {
			return proxy.SetupSOCKSExternalWithUsers(users)
		}
	}
	if len(cfg.Users) > 0 {
		return proxy.SetupSOCKSWithUsers(users)
	}
	return proxy.SetupSOCKS()
}

// pendingRevocationNotes describes access removals not confirmed yet.
func pendingRevocationNotes() []string {
	var notes []string
	for _, p := range coreinstall.PendingRevocations() {
		notes = append(notes, "Access removal pending on core "+p)
	}
	return notes
}

// userAccessSummary names what a new user can connect with, for the message
// after the user is added.
func userAccessSummary(u config.UserConfig) string {
	if u.MeteredOnly() {
		return "VLESS, REALITY and XHTTP only"
	}
	return "SSH + SOCKS"
}
