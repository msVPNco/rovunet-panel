package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	SubscriptionV2Socket = "/run/rovunet-subscription-v2/http.sock"
	DefaultConfigDir     = "/etc/rovunet"
	DefaultConfigFile    = "/etc/rovunet/config.json"
	DefaultTunnelDir     = "/etc/rovunet/tunnels"
	DefaultBinDir        = "/usr/local/bin"
	SystemUser           = "rovunet"
	SystemGroup          = "rovunet"
	SSHGroup             = "rovunet-ssh"
)

// WarpConfig tracks Cloudflare WARP outbound state.
type WarpConfig struct {
	Enabled bool `json:"enabled"`
}

// Config is the top-level rovunet configuration.
type Config struct {
	loaded   bool
	diskSHA  [32]byte
	mu       sync.RWMutex
	path     string
	Listen   ListenConfig    `json:"listen"`
	Tunnels  []TunnelConfig  `json:"tunnels"`
	Backends []BackendConfig `json:"backends"`
	Users    []UserConfig    `json:"users,omitempty"`
	// Archived holds users removed for expiry or traffic; see ArchivedUser.
	Archived []ArchivedUser `json:"archived,omitempty"`
	Route    RouteConfig    `json:"route"`
	Warp     WarpConfig     `json:"warp,omitempty"`
	// PublicAddress is the IP or hostname client configs use to reach this
	// server; empty means detect it (see clientcfg.ServerAddress).
	PublicAddress string `json:"public_address,omitempty"`
	// ExtraAddresses are more IPs or hostnames that reach this same server.
	// Subscription feeds repeat every address-based config once per extra
	// address, so when one address is blocked the app tries the next.
	ExtraAddressIDs map[string]string `json:"extra_address_ids,omitempty"`
	ExtraAddresses  []string          `json:"extra_addresses,omitempty"`
	// ServerID names this panel in each user's status file. It is random,
	// made once, and does not change with the server's address or name.
	ServerID              string             `json:"server_id,omitempty"`
	V2Authority           *RevisionAuthority `json:"subscription_v2_authority,omitempty"`
	SubscriptionV2        bool               `json:"subscription_v2,omitempty"`
	InstanceID            string             `json:"instance_id,omitempty"`
	RecoverySealOperation string             `json:"recovery_seal_operation,omitempty"`
	RecoverySealed        bool               `json:"recovery_sealed,omitempty"`
	RouterLimits          RouterLimits       `json:"router_limits,omitzero"`
}

// UserConfig tracks a managed user (same credentials for SSH + SOCKS).
type SQLiteBinding struct {
	Directory string `json:"directory"`
	Store     string `json:"store"`
}

type UserConfig struct {
	SQLiteMeter *SQLiteBinding `json:"sqlite_meter,omitempty"`
	Username    string         `json:"username"`
	Password    string         `json:"password"`
	// ExpiresAt is when the user is removed (RFC 3339, UTC); "" means never.
	ExpiresAt string `json:"expires_at,omitempty"`
	// QuotaGB is the traffic limit in GB; 0 means unlimited.
	QuotaGB int `json:"quota_gb,omitempty"`
	// SubToken is the unguessable name of the user's subscription file. It is
	// replaced when the password changes, which retires the old link.
	SubToken string `json:"sub_token,omitempty"`
	// UsageBase is the traffic already counted for this name when the user
	// was created or renewed; the limit counts what is used beyond it.
	UsageBase int64 `json:"usage_base,omitempty"`
	// AccountID names the account in its status file. It is random, made
	// when the user is created, and kept through renewals and password
	// changes; a user removed and added again gets a new one.
	AccountID string `json:"account_id,omitempty"`
	// PeriodID names the current paid period: renewing the user starts a new
	// one, so the app never adds the traffic of two periods together.
	PeriodID string `json:"period_id,omitempty"`
}

// ArchivedUser is a user who was removed because the expiry passed or the
// traffic limit was reached. The record keeps the password and link, so
// renewing restores the same configs.
type ArchivedUser struct {
	UserConfig
	Reason string `json:"reason"` // "expired" or "traffic limit"
	At     string `json:"at"`     // RFC 3339
}

// QuotaBytes is the traffic limit in bytes (1 GB = 1024^3), or 0 for none.
func (u UserConfig) QuotaBytes() int64 { return int64(u.QuotaGB) << 30 }

// MeteredOnly reports whether the user's traffic limit confines them to the
// paths whose traffic the panel counts: VLESS (WebSocket), REALITY and XHTTP.
// Every other service (SSH, NaïveProxy, SOCKS5, Hysteria 2, WireGuard, the
// DNS tunnels) would let them use traffic that is never counted, so they must
// not hold a credential there. This is the one rule every service follows:
// hiding a config is not enough, the server must refuse the user.
func (u UserConfig) MeteredOnly() bool { return u.QuotaGB > 0 }

// UnmeteredUsers returns the users who may hold a credential on a service
// that does not count traffic.
func UnmeteredUsers(users []UserConfig) []UserConfig {
	out := make([]UserConfig, 0, len(users))
	for _, u := range users {
		if !u.MeteredOnly() {
			out = append(out, u)
		}
	}
	return out
}

// NewSubToken returns a fresh 128-bit random subscription token (32 hex).
func NewSubToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// NewID returns a fresh random identifier (128 bits, 32 hex) for a server,
// an account or a period. It is the same shape as a subscription token but
// is never one: anyone with the subscription link can read it, and it opens
// nothing.
func NewID() string { return NewSubToken() }

// Expired reports whether the user has an expiry that has passed.
func (u UserConfig) Expired(now time.Time) bool {
	if u.ExpiresAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, u.ExpiresAt)
	if err != nil {
		return false // an unreadable date never deletes anyone
	}
	return !now.Before(t)
}

// ListenConfig defines the DNS listen address.
type ListenConfig struct {
	Address string `json:"address"`
}

// RouteConfig defines routing behavior.
type RouteConfig struct {
	Mode    string `json:"mode"`    // "single" or "multi"
	Active  string `json:"active"`  // active tunnel tag (single mode)
	Default string `json:"default"` // default tunnel tag (multi mode fallback)
}

// Load reads config from the default path.
func Load() (*Config, error) {
	return LoadFrom(DefaultConfigFile)
}

// LoadFrom reads config from a specific path.
func LoadFrom(path string) (*Config, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.path = path
	cfg.loaded = true
	cfg.diskSHA = sha256.Sum256(data)
	return &cfg, nil
}

// Save writes the config to disk.
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	path := c.path
	if path == "" {
		path = DefaultConfigFile
	}
	return c.saveLocked(path, false)
}

// SaveTo explicitly chooses the destination (including authorized restore).
// Later writes to the loaded destination reject stale concurrent snapshots.
func (c *Config) SaveTo(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked(path, true)
}

// Path returns the file the config was loaded from (or will be saved to).
func (c *Config) Path() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.path
}

// Reload refreshes configuration without copying the in-process mutex.
func (c *Config) Reload() error {
	fresh, err := LoadFrom(c.Path())
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replaceLocked(fresh)
	return nil
}

// Default returns a new config with sensible defaults.
func Default() *Config {
	return &Config{
		path: DefaultConfigFile,
		Listen: ListenConfig{
			Address: "0.0.0.0:53",
		},
		Backends: []BackendConfig{
			{Tag: "socks", Type: BackendSOCKS, Address: "127.0.0.1:1080", SOCKS: &SOCKSConfig{}},
			{Tag: "ssh", Type: BackendSSH, Address: "127.0.0.1:22"},
		},
		Route: RouteConfig{
			Mode: "single",
		},
	}
}

// GetTunnel returns a tunnel by tag.
// VLESSFront returns the NaiveProxy tunnel that fronts a VLESS tunnel, or
// nil when the tunnel is not fronted (or its front no longer exists).
func (c *Config) VLESSFront(t *TunnelConfig) *TunnelConfig {
	if t == nil || t.VLESS == nil || t.VLESS.Front == "" {
		return nil
	}
	f := c.GetTunnel(t.VLESS.Front)
	if f == nil || f.Transport != TransportNaive || f.Naive == nil {
		return nil
	}
	return f
}

// StunTLSFront returns the NaiveProxy tunnel that fronts a StunTLS tunnel.
func (c *Config) StunTLSFront(t *TunnelConfig) *TunnelConfig {
	if t == nil || t.StunTLS == nil || t.StunTLS.Front == "" {
		return nil
	}
	f := c.GetTunnel(t.StunTLS.Front)
	if f == nil || f.Transport != TransportNaive || f.Naive == nil {
		return nil
	}
	return f
}

func (c *Config) GetTunnel(tag string) *TunnelConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.Tunnels {
		if c.Tunnels[i].Tag == tag {
			return &c.Tunnels[i]
		}
	}
	return nil
}

// UniqueTag returns a tag that doesn't conflict with existing tunnels.
// If base is available it is returned as-is, otherwise a numeric suffix is appended.
func (c *Config) UniqueTag(base string) string {
	if c.GetTunnel(base) == nil {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if c.GetTunnel(candidate) == nil {
			return candidate
		}
	}
}

// AddTunnel adds a tunnel to the config.
func (c *Config) AddTunnel(t TunnelConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Tunnels = append(c.Tunnels, t)
}

// UpdateTunnel replaces a tunnel config by tag.
func (c *Config) UpdateTunnel(t TunnelConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Tunnels {
		if c.Tunnels[i].Tag == t.Tag {
			c.Tunnels[i] = t
			return
		}
	}
}

// RemoveTunnel removes a tunnel by tag.
func (c *Config) RemoveTunnel(tag string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Tunnels {
		if c.Tunnels[i].Tag == tag {
			c.Tunnels = append(c.Tunnels[:i], c.Tunnels[i+1:]...)
			return true
		}
	}
	return false
}

// GetBackend returns a backend by tag.
func (c *Config) GetBackend(tag string) *BackendConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.Backends {
		if c.Backends[i].Tag == tag {
			return &c.Backends[i]
		}
	}
	return nil
}

// ServerHostnames returns the DNS names that must point at this server for
// its tunnels: ns.<parent domain> for DNS tunnels and the domain itself for
// NaiveProxy.
func (c *Config) ServerHostnames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	seen := make(map[string]bool)
	var names []string
	for _, t := range c.Tunnels {
		var name string
		switch {
		case t.Domain == "":
			continue
		case t.Transport == TransportNaive:
			name = t.Domain
		case t.IsDNSTunnel():
			name = "ns." + ParentDomain(t.Domain)
		default:
			continue
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// ParentDomain returns the domain a subdomain belongs to, e.g.
// "t.example.com" → "example.com". Two-label domains are returned as is.
func ParentDomain(domain string) string {
	parts := strings.FieldsFunc(domain, func(r rune) bool { return r == '.' })
	if len(parts) <= 2 {
		return domain
	}
	return strings.Join(parts[1:], ".")
}

// GetUser returns a user by username.
func (c *Config) GetUser(username string) *UserConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.Users {
		if c.Users[i].Username == username {
			return &c.Users[i]
		}
	}
	return nil
}

// AddUser adds a user to the config. If a user with the same username
// already exists, it is updated in place instead of creating a duplicate.
func (c *Config) AddUser(u UserConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Users {
		if c.Users[i].Username == u.Username {
			c.Users[i] = u
			return
		}
	}
	c.Users = append(c.Users, u)
}

// RemoveUser removes a user by username.
func (c *Config) RemoveUser(username string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.Users {
		if c.Users[i].Username == username {
			c.Users = append(c.Users[:i], c.Users[i+1:]...)
			return true
		}
	}
	return false
}

// TunnelDir returns the directory for a tunnel's files.
func TunnelDir(tag string) string {
	return filepath.Join(DefaultTunnelDir, tag)
}

// RevisionAuthority survives disable and loss of the private registry.
type RevisionAuthority struct {
	ID     string `json:"id"`
	Server string `json:"server"`
	State  string `json:"state"`
}
