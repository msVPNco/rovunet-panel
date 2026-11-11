package coreinstall

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/network"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

func waitReady(name string) error {
	m, err := ReadMeta(name)
	if err != nil {
		return err
	}
	for deadline := time.Now().Add(6 * time.Second); time.Now().Before(deadline); {
		active, err := service.Status(ServiceName(name))
		ready := (err == nil && active == "active") || active == "activating"
		for _, proto := range strings.Split(m.Proto, ",") {
			flag := "-Hlnt"
			if proto == "udp" {
				flag = "-Hlnu"
			}
			b, err := exec.Command("ss", flag, "sport", "=", ":"+strconv.Itoa(m.Port)).Output()
			ready = ready && err == nil && strings.TrimSpace(string(b)) != ""
		}
		if m.Protocol == "amneziawg" {
			ready = ready && awgListening(name, m.Port)
		}
		if ready {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("core %s did not become active with all required listeners", name)
}

func Start(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	m, err := ReadMeta(name)
	if err != nil {
		return err
	}
	dir := filepath.Join(Root, name)
	_, st, managed, err := loadUserCore(name)
	if err != nil {
		return err
	}
	if managed && st.Paused {
		return fmt.Errorf("core has no eligible accounts")
	}
	if active, _ := service.Status(ServiceName(name)); active != "active" {
		if err := PortFree(m.Port, m.Proto); err != nil {
			return err
		}
	}
	if err := ensureCoreEngine(name, m.Protocol); err != nil {
		return err
	}
	if err := TestConfig(filepath.Join(dir, "server.json")); err != nil {
		return err
	}
	if err := network.AllowOwnedPort("core:"+name, m.Port, m.Proto); err != nil {
		return err
	}
	if err := service.Create(Unit(name, dir)); err != nil {
		return err
	}
	if managed && st.Disabled {
		st.Disabled = false
		if err := saveState(dir, &st); err != nil {
			return err
		}
	}
	if err := service.Start(ServiceName(name)); err != nil {
		return err
	}
	if err := waitReady(name); err != nil {
		_ = service.Stop(ServiceName(name))
		return err
	}
	if managed {
		// ExecCondition/ExecStartPost may have updated state while starting.
		// Never overwrite those changes with the pre-start snapshot.
		if err := Confirm(name); err != nil {
			return err
		}
	}
	RecordCert(name)
	return EnsureCertTimer()
}

func Stop(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if _, err := ReadMeta(name); err != nil {
		return err
	}
	_ = collectCore(name)
	if err := service.Stop(ServiceName(name)); err != nil {
		return err
	}
	_, st, managed, err := loadUserCore(name)
	if err != nil {
		return err
	}
	if managed {
		st.Disabled = true
		return saveState(filepath.Join(Root, name), &st)
	}
	return nil
}

func Confirm(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := waitReady(name); err != nil {
		return err
	}
	dir := filepath.Join(Root, name)
	_, st, managed, err := loadUserCore(name)
	if err != nil {
		return err
	}
	if managed {
		if st.APISocket != "" {
			if err := os.Chmod(st.APISocket, 0600); err != nil {
				return err
			}
		}
		st.ApplyError = ""
		st.Revocation = ""
		if err := saveState(dir, &st); err != nil {
			return err
		}
	}
	return saveApplied(dir)
}

// ChangePort updates only this core and restores its previous files and
// service if validation, listeners or startup fail. Keys and account IDs stay.
func ChangePort(name string, port int, users []config.UserConfig) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	o, st, managed, err := loadUserCore(name)
	if err != nil {
		return err
	}
	if !managed {
		return fmt.Errorf("port changes require an account-managed core")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be 1..65535")
	}
	if port == o.Port {
		return nil
	}
	if err := PortFree(port, Proto(o.Protocol)); err != nil {
		return err
	}
	dir := filepath.Join(Root, name)
	before, err := snapshotApplied(dir)
	if err != nil {
		return err
	}
	oldOptions, oldState := o, st
	oldApplied, appliedErr := os.ReadFile(filepath.Join(dir, appliedFile))
	wasActive, _ := service.Status(ServiceName(name))
	rollback := func(cause error) error {
		_, restoreErr := writeUserCore(dir, oldOptions, &oldState, users)
		if restoreErr == nil {
			restoreErr = os.WriteFile(filepath.Join(dir, "server.json"), before.Server, 0600)
		}
		if appliedErr == nil {
			_ = os.WriteFile(filepath.Join(dir, appliedFile), oldApplied, 0600)
		}
		if wasActive == "active" {
			restoreErr = errors.Join(restoreErr, service.Restart(ServiceName(name)), waitReady(name))
		}
		restoreErr = errors.Join(restoreErr, network.ReleaseOwnedPort("core:"+name, port, Proto(o.Protocol)))
		return errors.Join(cause, restoreErr)
	}
	o.Port = port
	if _, err := writeUserCore(dir, o, &st, users); err != nil {
		return rollback(err)
	}
	if err := TestConfig(filepath.Join(dir, "server.json")); err != nil {
		return rollback(err)
	}
	if err := network.AllowOwnedPort("core:"+name, port, Proto(o.Protocol)); err != nil {
		return rollback(err)
	}
	if wasActive == "active" {
		if err := service.Restart(ServiceName(name)); err != nil {
			return rollback(err)
		}
		if err := waitReady(name); err != nil {
			return rollback(err)
		}
	}
	if err := saveApplied(dir); err != nil {
		return rollback(err)
	}
	return network.ReleaseOwnedPort("core:"+name, oldOptions.Port, Proto(o.Protocol))
}
