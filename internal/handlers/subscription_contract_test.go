package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/transport"
)

// This defends the existing cross-repository strict v1 reader boundary.
// It is server-output verification, not an execution of a client importer.
func TestGeneratedSubscriptionsKeepStrictV1Contract(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	addManagedSubscriptionFixture(t, cfg)
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".json", ".windows.json"} {
		data, err := os.ReadFile(filepath.Join(transport.SubscriptionDir, "sub", cfg.Users[0].SubToken+suffix))
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal(data, &root); err != nil {
			t.Fatal(err)
		}
		if len(root) != 3 || string(root["version"]) != "1" || root["name"] == nil || root["items"] == nil {
			t.Fatal("generated feed violates the strict reader's root contract")
		}
		var entries []map[string]json.RawMessage
		if err := json.Unmarshal(root["items"], &entries); err != nil {
			t.Fatal(err)
		}
		if len(entries) < 1 || len(entries) > 1000 {
			t.Fatal("invalid item count")
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			if len(entry) != 2 || entry["id"] == nil || entry["link"] == nil {
				t.Fatal("unexpected item fields")
			}
			var id, link string
			if json.Unmarshal(entry["id"], &id) != nil || json.Unmarshal(entry["link"], &link) != nil ||
				strings.TrimSpace(id) == "" || len([]rune(id)) > 200 || seen[id] {
				t.Fatal("invalid or duplicate item identity")
			}
			compatibleExtra := suffix == ".windows.json" &&
				(id == "managed-awg" && strings.HasPrefix(link, "wireguard://") ||
					id == "managed-ss2022" && strings.HasPrefix(link, "ss://"))
			if !strings.HasPrefix(link, "rovunet://") && !compatibleExtra {
				t.Fatal("unexpected profile encoding")
			}
			seen[id] = true
		}
	}
}

func TestTooManyAddressProfilesRetainPreviouslyValidFeed(t *testing.T) {
	cfg, _ := twentyProfileSubscriptionFixture(t)
	addManagedSubscriptionFixture(t, cfg)
	if err := WriteSubscriptions(cfg, discardOutput{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(transport.SubscriptionDir, "sub", cfg.Users[0].SubToken+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 150; i++ {
		cfg.ExtraAddresses = append(cfg.ExtraAddresses, fmt.Sprintf("198.51.100.%d", i))
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if err := WriteSubscriptions(cfg, discardOutput{}); err == nil {
		t.Fatal("published oversized feed")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("oversized expansion replaced or deleted the last valid subscription")
	}
}
