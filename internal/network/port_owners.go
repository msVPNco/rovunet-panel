package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var ownerFile = "/etc/rovunet/firewall-owners.json"

type portOwnership struct {
	Owners      map[string]bool `json:"owners"`
	Preexisting bool            `json:"preexisting"`
}

var ownedAllow = AllowPort
var ownedRemove = RemovePort
var ownedRuleExists = func(port int, proto string) bool {
	// Keep rules from other firewall managers unless exclusivity is proven.
	if ufwActive() || firewalldActive() {
		return true
	}
	return exec.Command("iptables", "-C", "INPUT", "-p", proto, "--dport", strconv.Itoa(port), "-j", "ACCEPT").Run() == nil
}

func withPortOwners(f func(map[string]portOwnership) error) error {
	if err := os.MkdirAll(filepath.Dir(ownerFile), 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(ownerFile+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	owners := map[string]portOwnership{}
	if b, err := os.ReadFile(ownerFile); err == nil {
		if err := json.Unmarshal(b, &owners); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	// Persist completed operations even when a later protocol fails. Otherwise
	// a TCP success followed by a UDP failure would orphan the first rule.
	operationErr := f(owners)
	b, err := json.Marshal(owners)
	if err != nil {
		return errors.Join(operationErr, err)
	}
	if err := os.WriteFile(ownerFile+".tmp", b, 0600); err != nil {
		return errors.Join(operationErr, err)
	}
	return errors.Join(operationErr, os.Rename(ownerFile+".tmp", ownerFile))
}

// AllowOwnedPort records whether this core created the rule. Existing and
// shared rules are retained on remove/port changes, preserving other services.
func AllowOwnedPort(owner string, port int, proto string) error {
	if owner == "" {
		return fmt.Errorf("firewall owner required")
	}
	return withPortOwners(func(owners map[string]portOwnership) error {
		for _, p := range strings.Split(proto, ",") {
			key := fmt.Sprintf("%d/%s", port, p)
			entry, ok := owners[key]
			if !ok {
				entry = portOwnership{Owners: map[string]bool{}, Preexisting: ownedRuleExists(port, p)}
			}
			if entry.Owners == nil {
				entry.Owners = map[string]bool{}
			}
			err := ownedAllow(port, p)
			// AllowPort may add a live rule and then fail to persist it.
			if err == nil || ownedRuleExists(port, p) {
				entry.Owners[owner] = true
				owners[key] = entry
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func ReleaseOwnedPort(owner string, port int, proto string) error {
	return withPortOwners(func(owners map[string]portOwnership) error {
		for _, p := range strings.Split(proto, ",") {
			key := fmt.Sprintf("%d/%s", port, p)
			entry, ok := owners[key]
			if !ok || !entry.Owners[owner] {
				continue
			}
			if len(entry.Owners) == 1 && !entry.Preexisting {
				// A different live service may have begun sharing the port.
				if PortFree(port, p) != nil {
					entry.Preexisting = true
				} else if err := ownedRemove(port, p); err != nil {
					return err
				}
			}
			delete(entry.Owners, owner)
			if len(entry.Owners) == 0 && !entry.Preexisting {
				delete(owners, key)
			} else {
				owners[key] = entry
			}
		}
		return nil
	})
}
