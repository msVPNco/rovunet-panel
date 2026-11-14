package transport

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLegacySubscriptionRouteTrustAndContainment(t *testing.T) {
	route := subscriptionRouteLines()
	var pattern string
	for _, line := range strings.Split(route, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "path_regexp" {
			pattern = fields[1]
		}
		// The old one-line matcher is retained for the baseline regression.
		if len(fields) == 3 && fields[1] == "path_regexp" {
			pattern = fields[2]
		}
	}
	if pattern == "" {
		t.Fatal("missing subscription matcher")
	}
	matcher := regexp.MustCompile(pattern)
	token := strings.Repeat("a1", 16)
	for _, suffix := range []string{".json", ".windows.json", ".status.json", ".windows.status.json", ".trust.json"} {
		if !matcher.MatchString("/sub/" + token + suffix) {
			t.Errorf("valid resource rejected: %s", suffix)
		}
	}
	for _, path := range []string{
		"/sub/" + token + ".windows.trust.json",
		"/sub/" + token + ".trust.status.json",
		"/sub/" + token + ".trust.json.bak",
		"/sub/" + token + ".trust.json/extra",
		"/sub/" + token[:30] + ".trust.json",
		"/sub/" + strings.ToUpper(token) + ".trust.json",
		"/sub/../" + token + ".trust.json",
		"/sub/%2e%2e/" + token + ".trust.json",
		"/sub/" + token + "%2etrust.json",
		"/sub/" + token + ".private.json",
		"/sub/", "/v2/sub/" + token + ".trust.json",
	} {
		if matcher.MatchString(path) {
			t.Errorf("unexpected resource exposed: %s", path)
		}
	}
	if !strings.Contains(route, `Cache-Control "no-store"`) || strings.Contains(route, "browse") {
		t.Fatal("cache or listing policy changed")
	}
	// Optional fixture for testing the exact emitted route with the shipped Caddy.
	if dir := os.Getenv("ROVUNET_LEGACY_TRUST_ROUTE_FIXTURE"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "route.txt"), []byte(route), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
