package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FiniteUpgradePending prevents reboot from re-admitting a legacy path in the
// middle of a cold accounting migration. Ready means all new configs validated.
func FiniteUpgradePending(path, user string) bool {
	b, e := os.ReadFile(filepath.Join(path+".finite-upgrade", "pending.json"))
	if os.IsNotExist(e) {
		return false
	}
	if e != nil {
		return true
	}
	var p struct {
		Version int
		Phase   string
		Users   []string
	}
	if json.Unmarshal(b, &p) != nil || p.Version != 1 || (p.Phase != "planned" && p.Phase != "ready" && p.Phase != "complete") {
		return true
	}
	if p.Phase == "ready" || p.Phase == "complete" {
		return false
	}
	if user == "" {
		return true
	}
	if len(p.Users) == 0 {
		return true
	}
	for _, name := range p.Users {
		if name == user {
			return true
		}
	}
	return false
}

// A cutover must not suppress unrelated unlimited-only cores at reboot.
func FiniteUpgradeCorePending(path, name string) bool {
	b, e := os.ReadFile(filepath.Join(path+".finite-upgrade", "pending.json"))
	if os.IsNotExist(e) {
		return false
	}
	if e != nil {
		return true
	}
	var p struct {
		Version int
		Phase   string
		Cores   []string
	}
	if json.Unmarshal(b, &p) != nil || p.Version != 1 || (p.Phase != "planned" && p.Phase != "ready" && p.Phase != "complete") {
		return true
	}
	if p.Phase != "planned" {
		return false
	}
	for _, core := range p.Cores {
		if core == name {
			return true
		}
	}
	return false
}
