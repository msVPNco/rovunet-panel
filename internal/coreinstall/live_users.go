package coreinstall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// These protocols have a per-user manager in the pinned Xray. rmu rejects
// future sessions; that API does not promise to terminate established streams.
func liveUserProtocol(protocol string) bool {
	return protocol == "ss2022" || protocol == "trojan" || protocol == "vlessenc"
}

type liveConfig struct {
	Inbounds []struct {
		Tag      string `json:"tag"`
		Protocol string `json:"protocol"`
		Settings struct {
			Clients []map[string]any `json:"clients"`
		} `json:"settings"`
	} `json:"inbounds"`
}

var runUserAPI = func(endpoint string, action string, args ...string) ([]byte, error) {
	base := []string{"api", action, "-s", endpoint, "-t", "5"}
	return exec.Command(XrayPath, append(base, args...)...).CombinedOutput()
}

// liveUsers applies only credential changes without restarting healthy users.
// Both the disk config (next boot) and API must converge before success.
func liveUsers(name string, previous []byte, path string) (bool, error) {
	o, st, ok, err := loadUserCore(name)
	if err != nil || !ok || !liveUserProtocol(o.Protocol) || apiEndpoint(st) == "" {
		return false, err
	}
	next, err := os.ReadFile(path)
	if err != nil {
		return true, err
	}
	var oldDoc, newDoc liveConfig
	if json.Unmarshal(previous, &oldDoc) != nil || json.Unmarshal(next, &newDoc) != nil || len(oldDoc.Inbounds) == 0 || len(newDoc.Inbounds) == 0 {
		return true, fmt.Errorf("cannot compare applied users")
	}
	before, after := map[string]map[string]any{}, map[string]map[string]any{}
	for _, u := range oldDoc.Inbounds[0].Settings.Clients {
		email, _ := u["email"].(string)
		before[email] = u
	}
	for _, u := range newDoc.Inbounds[0].Settings.Clients {
		email, _ := u["email"].(string)
		after[email] = u
	}
	equal := func(a, b any) bool { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return bytes.Equal(x, y) }
	// A transport, TLS or port change requires an explicit lifecycle change.
	var oldRoot, newRoot map[string]any
	_ = json.Unmarshal(previous, &oldRoot)
	_ = json.Unmarshal(next, &newRoot)
	for _, root := range []map[string]any{oldRoot, newRoot} {
		ins, ok := root["inbounds"].([]any)
		if !ok || len(ins) == 0 {
			return true, fmt.Errorf("invalid applied inbound")
		}
		in, ok := ins[0].(map[string]any)
		if !ok {
			return true, fmt.Errorf("invalid applied inbound")
		}
		settings, ok := in["settings"].(map[string]any)
		if !ok {
			return true, fmt.Errorf("invalid applied credentials")
		}
		delete(settings, "clients")
	}
	if !equal(oldRoot, newRoot) {
		return false, nil
	}
	if err := testXray(path); err != nil {
		return true, err
	}
	users, err := runUserAPI(apiEndpoint(st), "inbounduser", "-tag=proxy")
	if err != nil {
		return true, fmt.Errorf("cannot confirm current engine users")
	}
	var existing struct {
		Users []struct {
			Email string `json:"email"`
		} `json:"users"`
	}
	if json.Unmarshal(users, &existing) != nil {
		return true, fmt.Errorf("cannot read engine user state")
	}
	present := map[string]bool{}
	for _, u := range existing.Users {
		present[u.Email] = true
	}
	changed := map[string]bool{}
	for email, u := range before {
		if !equal(u, after[email]) {
			changed[email] = true
		}
	}
	for email, u := range after {
		if !equal(u, before[email]) {
			changed[email] = true
		}
	}
	for email := range changed {
		if !present[email] {
			continue
		}
		out, err := runUserAPI(apiEndpoint(st), "rmu", "-tag=proxy", email)
		if err != nil || !strings.Contains(string(out), "Removed 1 user(s) in total.") {
			return true, fmt.Errorf("per-user removal was not confirmed")
		}
	}
	var additions []map[string]any
	for email, u := range after {
		if !equal(u, before[email]) {
			additions = append(additions, u)
		}
	}
	if len(additions) == 0 {
		return true, nil
	}
	var doc map[string]any
	_ = json.Unmarshal(next, &doc)
	in := doc["inbounds"].([]any)[0].(map[string]any)
	in["settings"].(map[string]any)["clients"] = additions
	doc["inbounds"] = []any{in}
	file, err := os.CreateTemp(filepath.Dir(path), ".user-api-*.json")
	if err != nil {
		return true, err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	b, _ := json.Marshal(doc)
	_, err = file.Write(b)
	closeErr := file.Close()
	if err != nil {
		return true, err
	}
	if closeErr != nil {
		return true, closeErr
	}
	out, err := runUserAPI(apiEndpoint(st), "adu", tmp)
	if err != nil || !strings.Contains(string(out), fmt.Sprintf("Added %d user(s) in total.", len(additions))) {
		return true, fmt.Errorf("per-user addition was not confirmed")
	}
	return true, nil
}
