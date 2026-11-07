package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestParseLimit(t *testing.T) {
	for _, s := range []string{"", " ", "0", "free", "Unlimited"} {
		if n, err := parseLimit(s, 100); err != nil || n != 0 {
			t.Errorf("%q: got %d, %v; want unlimited", s, n, err)
		}
	}
	if n, err := parseLimit("30", 100); err != nil || n != 30 {
		t.Errorf("30: got %d, %v", n, err)
	}
	for _, s := range []string{"-1", "abc", "101", "1.5"} {
		if _, err := parseLimit(s, 100); err == nil {
			t.Errorf("%q must be rejected", s)
		}
	}
}

func TestAskUserLimitsFromArguments(t *testing.T) {
	days, gb, err := askUserLimits("30", "50")
	if err != nil || days != 30 || gb != 50 {
		t.Fatalf("got %d %d %v", days, gb, err)
	}
	days, gb, err = askUserLimits("", "20")
	if err != nil || days != 0 || gb != 20 {
		t.Fatalf("gb only: got %d %d %v", days, gb, err)
	}
	if _, _, err := askUserLimits("x", ""); err == nil || !strings.Contains(err.Error(), "days") {
		t.Fatalf("bad days: %v", err)
	}
}

func TestNewUserExpiry(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	free := newUser("a", "p", 0, 0, now)
	if free.ExpiresAt != "" || free.QuotaGB != 0 || free.Expired(now.Add(1000*time.Hour)) {
		t.Fatalf("unlimited user must never expire: %+v", free)
	}
	if got := limitSummary(free); got != "unlimited" {
		t.Fatalf("summary %q", got)
	}
	u := newUser("b", "p", 30, 20, now)
	if u.ExpiresAt != "2026-12-07T12:00:00Z" || u.QuotaGB != 20 {
		t.Fatalf("got %+v", u)
	}
	if u.Expired(now.Add(29*24*time.Hour)) || !u.Expired(now.Add(30*24*time.Hour)) {
		t.Fatal("expiry must fall exactly 30 days later")
	}
	if got := limitSummary(u); got != "expires 2026-12-07 12:00 UTC, 20 GB" {
		t.Fatalf("summary %q", got)
	}
}

func TestUnreadableExpiryNeverDeletes(t *testing.T) {
	u := config.UserConfig{Username: "x", ExpiresAt: "garbage"}
	if u.Expired(time.Now().Add(100000 * time.Hour)) {
		t.Fatal("an unreadable date must not expire a user")
	}
}
