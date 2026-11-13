package coreinstall

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

var hy2CheckConfig = TestConfig
var hy2Status = service.Status
var hy2Restart = service.Restart
var hy2Ready = waitReady

// obfsSnapshot captures only files this operation changes. TLS keys, counters,
// other cores and panel account/subscription identities are outside the set.
func obfsSnapshot(dir string) (map[string][]byte, error) {
	files := []string{"server.json", optionsFile, stateFile, "meta", appliedFile}
	for _, p := range []string{dir, filepath.Join(dir, "clients")} {
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("core directory must not be a symlink")
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "clients"))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		files = append(files, filepath.Join("clients", e.Name()))
	}
	result := make(map[string][]byte, len(files))
	for _, name := range files {
		p := filepath.Join(dir, name)
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return nil, fmt.Errorf("unexpected private core file type or size")
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		result[name] = data
	}
	return result, nil
}

func writeObfsSnapshot(dir string, files map[string][]byte) error {
	if err := os.MkdirAll(filepath.Join(dir, "clients"), 0700); err != nil {
		return err
	}
	for name, data := range files {
		if err := safefile.Write(filepath.Join(dir, name), data, 0600); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "clients"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, exists := files[filepath.Join("clients", e.Name())]; !exists {
			if err := os.Remove(filepath.Join(dir, "clients", e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// ChangeHysteriaObfuscation changes only the selected account-managed Hy2 core.
// A private recovery copy precedes mutation; failed validation/startup restores
// its exact prior files and active state. A successful toggle requires clients
// to refresh their existing subscription URL; saved plain profiles cannot use a
// masked listener. Updates call it once for cores without an operator choice.
func ChangeHysteriaObfuscation(name string, enabled bool, users []config.UserConfig) (backup string, err error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	o, st, managed, err := loadUserCore(name)
	if err != nil {
		return "", err
	}
	if !managed || o.Protocol != "hysteria2" {
		return "", fmt.Errorf("requires an account-managed hysteria2 core")
	}
	if st.ApplyError != "" || st.Revocation != "" {
		return "", fmt.Errorf("resolve pending core changes before switching obfuscation")
	}
	dir := filepath.Join(Root, name)
	files, err := obfsSnapshot(dir)
	if err != nil {
		return "", err
	}
	prior, err := loadApplied(dir)
	if err != nil {
		return "", err
	}
	current, err := snapshotApplied(dir)
	if err != nil {
		return "", err
	}
	if !sameJSON(prior.Server, current.Server) || !maps.Equal(prior.Clients, current.Clients) {
		return "", fmt.Errorf("resolve unapplied core files before switching obfuscation")
	}
	o.CoreName = name
	priorState := st
	expected, err := generateUserBundle(o, &priorState, clientsFor(usersFor(o.Protocol, users)))
	if err != nil {
		return "", err
	}
	if !sameJSON(expected.Server, current.Server) {
		return "", fmt.Errorf("synchronize pending account or core options before switching obfuscation")
	}
	if o.HysteriaSalamander == enabled && o.HysteriaObfsConfigured {
		return "", nil
	}
	active, _ := hy2Status(ServiceName(name))
	if active != "active" && active != "inactive" && active != "failed" {
		return "", fmt.Errorf("core state is not stable: %s", active)
	}
	// Validate the complete desired server in a private file before changing
	// existing files or restarting a service.
	modeChanged := o.HysteriaSalamander != enabled
	o.HysteriaSalamander, o.HysteriaObfsConfigured = enabled, true
	trialState := st
	o.CoreName = name
	bundle, err := generateUserBundle(o, &trialState, clientsFor(usersFor(o.Protocol, users)))
	if err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(dir, ".hy2-obfs-check-*.json")
	if err != nil {
		return "", err
	}
	checkPath := temp.Name()
	defer os.Remove(checkPath)
	_, writeErr := temp.Write(bundle.Server)
	err = errors.Join(writeErr, temp.Close())
	if err != nil {
		return "", err
	}
	if err := hy2CheckConfig(checkPath); err != nil {
		return "", err
	}
	backupRoot := filepath.Join(filepath.Dir(Root), "core-obfs-backups")
	if err := os.Mkdir(backupRoot, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	info, err := os.Lstat(backupRoot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("recovery directory must be private and not a symlink")
	}
	backup, err = os.MkdirTemp(backupRoot, name+"-")
	if err != nil {
		return "", err
	}
	if err := writeObfsSnapshot(backup, files); err != nil {
		return backup, err
	}
	for name, data := range files {
		copy, err := os.ReadFile(filepath.Join(backup, name))
		if err != nil || !bytes.Equal(copy, data) {
			return backup, fmt.Errorf("private recovery copy could not be rechecked")
		}
	}
	rollback := func(cause error) error {
		restoreErr := writeObfsSnapshot(dir, files)
		if restoreErr == nil && active == "active" && modeChanged {
			restoreErr = errors.Join(hy2Restart(ServiceName(name)), hy2Ready(name))
		}
		return errors.Join(cause, restoreErr)
	}
	if _, err := writeUserCore(dir, o, &st, users); err != nil {
		return backup, rollback(err)
	}
	if active == "active" && modeChanged {
		if err := hy2Restart(ServiceName(name)); err != nil {
			return backup, rollback(err)
		}
		if err := hy2Ready(name); err != nil {
			return backup, rollback(err)
		}
	}
	if err := saveApplied(dir); err != nil {
		return backup, rollback(err)
	}
	return backup, nil
}

// ConfigureDefaultHysteriaObfuscation migrates only older, account-managed
// cores without an operator choice. Names, endpoints and per-account auth stay.
// A later explicit off is durable and is not undone by another update.
func ConfigureDefaultHysteriaObfuscation(users []config.UserConfig) error {
	cores, err := List()
	if err != nil {
		return err
	}
	var pending []error
	for _, core := range cores {
		if core.Protocol != "hysteria2" {
			continue
		}
		o, _, managed, err := loadUserCore(core.Name)
		if err != nil {
			pending = append(pending, err)
			continue
		}
		if !managed || o.HysteriaObfsConfigured {
			continue
		}
		if _, err := ChangeHysteriaObfuscation(core.Name, true, users); err != nil {
			pending = append(pending, fmt.Errorf("Hy2 %s migration retained its recovery state: %w", core.Name, err))
		}
	}
	return errors.Join(pending...)
}
