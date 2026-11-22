package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/clientexport"
	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestBuildDonationCandidate(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	u := config.UserConfig{
		Username: "gift-abc", AccountID: "donation-id", Donation: true,
		QuotaGB: 50, ExpiresAt: now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	p := []clientexport.Profile{{ID: "vless", Link: "vless://example-id@example.com:443?security=tls&type=ws"}}
	c, err := buildDonationCandidate(u, p, now)
	if err != nil { t.Fatal(err) }
	if c.Version != 1 || c.Kind != "rovunet-donation" || c.QuotaGB != 50 || len(c.Items) != 1 {
		t.Fatalf("unexpected donation candidate: %+v", c)
	}
	if c.CreatedAt != now.Format(time.RFC3339) || c.ExpiresAt != u.ExpiresAt || c.DonationID != u.AccountID {
		t.Fatalf("donation metadata or validity window changed: %+v", c)
	}
}

func TestBuildDonationCandidateRejectsUnsafeOrExpired(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	base := config.UserConfig{
		Username: "gift-abc", AccountID: "donation-id", Donation: true,
		QuotaGB: 50, ExpiresAt: now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	good := []clientexport.Profile{{ID: "vless", Link: "vless://example-id@example.com:443?security=tls"}}
	tests := []struct{
		name string
		user config.UserConfig
		profiles []clientexport.Profile
		want string
	}{
		{name:"unmarked account",user:func() config.UserConfig {u:=base;u.Donation=false;return u}(),profiles:good,want:"marked"},
		{name:"unlimited account",user:func() config.UserConfig {u:=base;u.QuotaGB=0;return u}(),profiles:good,want:"marked"},
		{name:"expired account",user:func() config.UserConfig {u:=base;u.ExpiresAt=now.Add(-time.Second).Format(time.RFC3339);return u}(),profiles:good,want:"expired"},
		{name:"no metered profiles",user:base,want:"no metered"},
		{name:"no plaintext VLESS",user:base,profiles:[]clientexport.Profile{{ID:"vless",Link:"vless://id@host:80?security=none"}},want:"insecure"},
		{name:"no insecure TLS",user:base,profiles:[]clientexport.Profile{{ID:"vless",Link:"vless://id@host:443?security=tls&allowInsecure=1"}},want:"insecure"},
		{name:"no raw private envelope",user:base,profiles:[]clientexport.Profile{{ID:"native",Link:"rovunet://BASE64"}},want:"compatible"},
	}
	for _, tt := range tests {
		t.Run(tt.name,func(t *testing.T) {
			_,err:=buildDonationCandidate(tt.user,tt.profiles,now)
			if err==nil || !strings.Contains(err.Error(),tt.want) { t.Fatalf("got %v, expected %q",err,tt.want) }
		})
	}
}
