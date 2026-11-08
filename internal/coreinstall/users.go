package coreinstall

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/network"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

// A user core serves the panel's users: every user gets credentials derived
// from their username and password, so user add, remove and passwd keep the
// core in step and a changed password revokes the old configuration. Cores
// installed before this (per-device files) have no options.json and are left
// exactly as they are.

const (
	optionsFile = "options.json"
	stateFile   = "state.json"
)

// Profile is one user's client profile on one core.
type Profile struct {
	Core, Protocol, Link string
}

// usersFor is who may hold a credential on a core of this protocol. Cores whose
// traffic the panel counts (REALITY, XHTTP) serve every user; the others
// (Hysteria 2, WireGuard) would let a traffic-limited user move traffic that is
// never counted, so those users are left out of them.
func usersFor(protocol string, users []config.UserConfig) []config.UserConfig {
	if coreplan.Metered(protocol) {
		return users
	}
	return config.UnmeteredUsers(users)
}

func clientsFor(users []config.UserConfig) []coreplan.Client {
	clients := make([]coreplan.Client, 0, len(users))
	for _, u := range users {
		clients = append(clients, coreplan.Client{Name: u.Username, Secret: u.Password})
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].Name < clients[j].Name })
	return clients
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// loadUserCore reads a user core's options and state; ok is false for a
// per-device core.
func loadUserCore(name string) (o coreplan.Options, st coreplan.State, ok bool, err error) {
	dir := filepath.Join(Root, name)
	if err = readJSON(filepath.Join(dir, optionsFile), &o); errors.Is(err, os.ErrNotExist) {
		return o, st, false, nil
	} else if err != nil {
		return o, st, false, err
	}
	if err = readJSON(filepath.Join(dir, stateFile), &st); err != nil {
		return o, st, false, err
	}
	coreplan.Normalize(&o) // repairs a hostname stored as a pasted markdown link
	return o, st, true, nil
}

// writeUserCore regenerates a user core's files for users. changed reports
// whether the server configuration differs from what was on disk.
func writeUserCore(dir string, o coreplan.Options, st *coreplan.State, users []config.UserConfig) (changed bool, err error) {
	clients := clientsFor(usersFor(o.Protocol, users))
	st.Paused = len(clients) == 0
	if st.APIPort == 0 && coreplan.Metered(o.Protocol) {
		st.APIPort = pickAPIPort(filepath.Base(dir))
	}
	b, err := coreplan.GenerateFor(o, st, clients)
	if err != nil {
		return false, err
	}
	serverPath := filepath.Join(dir, "server.json")
	old, _ := os.ReadFile(serverPath)
	changed = !bytes.Equal(old, b.Server)
	if changed {
		if err := os.WriteFile(serverPath, b.Server, 0600); err != nil {
			return false, err
		}
	}
	if err := writeJSON(filepath.Join(dir, optionsFile), o); err != nil {
		return false, err
	}
	if err := writeJSON(filepath.Join(dir, stateFile), st); err != nil {
		return false, err
	}
	meta := fmt.Sprintf("protocol=%s\nport=%d\nproto=%s\n", o.Protocol, o.Port, Proto(o.Protocol))
	if err := os.WriteFile(filepath.Join(dir, "meta"), []byte(meta), 0600); err != nil {
		return false, err
	}
	// Client files mirror the current users exactly.
	clientsDir := filepath.Join(dir, "clients")
	if err := os.RemoveAll(clientsDir); err != nil {
		return false, err
	}
	if err := os.Mkdir(clientsDir, 0700); err != nil {
		return false, err
	}
	for i, c := range clients {
		if config.ValidateUsername(c.Name) != nil {
			continue // never a path: only names the panel itself accepts
		}
		if err := os.WriteFile(filepath.Join(clientsDir, c.Name+"."+b.Extension), []byte(b.Clients[i]+"\n"), 0600); err != nil {
			return false, err
		}
	}
	return changed, nil
}

// InstallForUsers generates, validates and starts a new core serving users.
// On any failure everything it created is removed again.
func InstallForUsers(name string, o coreplan.Options, users []config.UserConfig) (dir string, err error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if err := coreplan.Validate(&o); err != nil {
		return "", err
	}
	proto := Proto(o.Protocol)
	if err := PortFree(o.Port, proto); err != nil {
		return "", err
	}
	st, err := coreplan.NewState()
	if err != nil {
		return "", err
	}
	if err := EnsureXray(); err != nil {
		return "", err
	}
	dir = filepath.Join(Root, name)
	if err := os.MkdirAll(Root, 0700); err != nil {
		return "", err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return "", fmt.Errorf("core already exists or cannot be created: %w", err)
	}
	unitName := ServiceName(name)
	defer func() {
		if err != nil {
			_ = service.Stop(unitName)
			_ = service.Remove(unitName)
			_ = os.RemoveAll(dir)
		}
	}()
	if _, err = writeUserCore(dir, o, &st, users); err != nil {
		return "", err
	}
	if len(users) > 0 && st.Paused {
		err = fmt.Errorf("no user may use %s: every user has a traffic limit, and only VLESS, REALITY and XHTTP are counted", o.Protocol)
		return "", err
	}
	if err = TestConfig(filepath.Join(dir, "server.json")); err != nil {
		return "", err
	}
	if err = service.Create(Unit(name, dir)); err != nil {
		return "", err
	}
	if err = service.Start(unitName); err != nil {
		return "", err
	}
	RecordCert(name)
	_ = EnsureCertTimer() // best effort; the post-update step retries it
	if ferr := network.AllowPort(o.Port, proto); ferr != nil {
		return dir, fmt.Errorf("core started, but opening %d/%s failed: %w", o.Port, proto, ferr)
	}
	return dir, nil
}

// Sync brings every user core in line with users. A core whose server
// configuration changed is validated by Xray and restarted; if Xray rejects
// it, the previous configuration is put back and the core keeps running.
func Sync(users []config.UserConfig) error {
	cores, err := List()
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range cores {
		if err := syncCore(c.Name, users); err != nil {
			errs = append(errs, fmt.Errorf("core %s: %w", c.Name, err))
		}
	}
	return errors.Join(errs...)
}

// Seams for tests.
var (
	startUnit           = service.Start
	stopUnit            = service.Stop
	testXray            = TestConfig
	xrayInstalled       = func() bool { _, err := os.Stat(XrayPath); return err == nil }
	waitBetweenAttempts = func() { time.Sleep(applyWait) }
)

const (
	// applyAttempts and applyWait bound how long a failed change is retried:
	// two attempts, 15 seconds apart.
	applyAttempts = 2
	applyWait     = 15 * time.Second
)

// clientNames lists whose client files a core currently has.
func clientNames(dir string) map[string]bool {
	names := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(dir, "clients"))
	if err != nil {
		return names
	}
	for _, e := range entries {
		names[strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))] = true
	}
	return names
}

func saveState(dir string, st *coreplan.State) error {
	return writeJSON(filepath.Join(dir, stateFile), st)
}

// applyOnce validates the new configuration and makes the core run it. retry
// is false when trying again cannot help (Xray rejected the file).
func applyOnce(name, serverPath string, start bool) (retry bool, err error) {
	if xrayInstalled() {
		if err := testXray(serverPath); err != nil {
			return false, err
		}
	}
	unit := ServiceName(name)
	if start {
		return true, startUnit(unit)
	}
	_ = collectCore(name) // Xray's counters restart at zero with the core
	return true, restartCore(unit)
}

// applyCore deploys a core's new configuration, trying at most applyAttempts
// times. When it still fails: a change that only adds users puts the previous
// configuration back and the core keeps running; a change that takes a user's
// access away stops the core, because running the old configuration would
// leave that user's access in place. The new file stays on disk either way.
func applyCore(name, serverPath string, previous []byte, revoking, start bool) error {
	var err error
	for attempt := 1; attempt <= applyAttempts; attempt++ {
		if attempt > 1 {
			waitBetweenAttempts()
		}
		var retry bool
		if retry, err = applyOnce(name, serverPath, start); err == nil {
			return nil
		} else if !retry {
			break
		}
	}
	if revoking {
		_ = stopUnit(ServiceName(name))
		return fmt.Errorf("access removal was not applied, so core %s was stopped: %w", name, err)
	}
	_ = os.WriteFile(serverPath, previous, 0600)
	return err
}

func syncCore(name string, users []config.UserConfig) error {
	o, st, ok, err := loadUserCore(name)
	if err != nil || !ok {
		return err
	}
	dir := filepath.Join(Root, name)
	serverPath := filepath.Join(dir, "server.json")
	previous, _ := os.ReadFile(serverPath)
	wasPaused, wasPending := st.Paused, st.Revocation != ""
	before := clientNames(dir)
	changed, err := writeUserCore(dir, o, &st, users)
	if err != nil {
		return err
	}
	after := clientNames(dir)
	revoking := wasPending
	for n := range before {
		if !after[n] {
			revoking = true
		}
	}

	if st.Paused {
		// No user may use this core. server.json holds no credential, so
		// stopping it ends every session; it is started again by the next
		// allowed user.
		if !wasPaused {
			_ = collectCore(name)
		}
		_ = stopUnit(ServiceName(name))
		st.Revocation = ""
		return saveState(dir, &st)
	}
	if !changed && !wasPaused && !wasPending {
		return nil
	}
	applyErr := applyCore(name, serverPath, previous, revoking, wasPaused || wasPending)
	if revoking {
		st.Revocation = ""
		if applyErr != nil {
			st.Revocation = applyErr.Error()
		}
		if err := saveState(dir, &st); err != nil && applyErr == nil {
			return err
		}
	}
	return applyErr
}

// PendingRevocations lists the cores where taking a user's access away has not
// been confirmed, as "core: reason". The limits timer retries them.
func PendingRevocations() []string {
	cores, err := List()
	if err != nil {
		return nil
	}
	var out []string
	for _, c := range cores {
		if _, st, ok, err := loadUserCore(c.Name); err == nil && ok && st.Revocation != "" {
			out = append(out, c.Name+": "+st.Revocation)
		}
	}
	return out
}

// Profiles returns user's profile on every user core, sorted by core name.
func Profiles(user config.UserConfig) ([]Profile, error) {
	cores, err := List()
	if err != nil {
		return nil, err
	}
	var out []Profile
	for _, c := range cores {
		o, st, ok, err := loadUserCore(c.Name)
		if err != nil {
			return out, fmt.Errorf("core %s: %w", c.Name, err)
		}
		if !ok {
			continue
		}
		// Generate for this user alone: credentials depend only on the user,
		// and the WireGuard address comes from the stored assignment. The
		// one-client generation prunes the address map, so give it a copy:
		// this read must never change the stored assignment.
		addrs := make(map[string]string, len(st.WGAddrs))
		for k, v := range st.WGAddrs {
			addrs[k] = v
		}
		st.WGAddrs = addrs
		b, err := coreplan.GenerateFor(o, &st, []coreplan.Client{{Name: user.Username, Secret: user.Password}})
		if err != nil {
			return out, fmt.Errorf("core %s: %w", c.Name, err)
		}
		out = append(out, Profile{Core: c.Name, Protocol: o.Protocol, Link: b.Clients[0]})
	}
	return out, nil
}
