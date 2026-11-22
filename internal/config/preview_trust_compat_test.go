package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPreviewTrustIdentitySurvivesConfigWrites(t *testing.T) {
	keyID := strings.Repeat("a", 64)
	fixture := map[string]any{
		"subscription_trust_key_id": keyID,
		"subscription_v2":           true,
		"subscription_v2_authority": map[string]any{"id": "authority", "server": "server", "state": "active"},
		"server_id":                 "server",
		"instance_id":               "instance",
		"public_address":            "192.0.2.10",
		"listen":                    map[string]any{"address": "0.0.0.0:53"},
		"tunnels": []any{map[string]any{
			"tag": "slipstream", "transport": "slipstream", "backend": "socks", "domain": "example.test", "port": 5311, "enabled": true,
			"slipstream": map[string]any{"cert": "/private/certificate.pem", "key": "/private/key.pem"},
		}},
		"users": []any{map[string]any{
			"username": "fixture", "password": "fixture-password", "sub_token": strings.Repeat("b", 32),
			"account_id": strings.Repeat("c", 32), "period_id": strings.Repeat("d", 32), "usage_base": 17,
			"sqlite_meter": map[string]any{"directory": "/private/accounting", "store": "existing-store"},
		}},
	}
	path := filepath.Join(t.TempDir(), "config.json")
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var original map[string]any
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SubscriptionTrustKeyID != keyID {
		t.Fatal("private preview trust identity was not loaded")
	}
	cfg.PublicAddress = "192.0.2.11"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	// Reload must copy the persisted compatibility field, not leave a stale
	// in-memory value or create a replacement identity.
	cfg.SubscriptionTrustKeyID = ""
	if err := cfg.Reload(); err != nil {
		t.Fatal(err)
	}
	if cfg.SubscriptionTrustKeyID != keyID {
		t.Fatal("reload lost the existing trust identity")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(after, &saved); err != nil {
		t.Fatal(err)
	}
	for name, before := range original {
		if name == "public_address" {
			continue
		}
		if !reflect.DeepEqual(saved[name], before) {
			t.Errorf("existing %s changed: got %v, want %v", name, saved[name], before)
		}
	}
	// An external removal must also be observed. Compatibility storage never
	// regenerates an identity or silently re-enables signing.
	current, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	current.SubscriptionTrustKeyID = ""
	if err := current.Save(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.RefreshIfChanged(); err != nil {
		t.Fatal(err)
	}
	if cfg.SubscriptionTrustKeyID != "" {
		t.Fatal("refresh resurrected removed trust identity metadata")
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	after, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "subscription_trust_key_id") {
		t.Fatal("an absent trust identity was generated or serialized")
	}
}
