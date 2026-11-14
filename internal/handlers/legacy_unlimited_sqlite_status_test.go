package handlers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

func TestLegacyUnlimitedSQLiteStatusKeepsV1ShapeAndLedger(t *testing.T) {
	for _, timed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unlimited", true: "time-only"}[timed], func(t *testing.T) {
			c, dir, _ := unlimitedFixture(t)
			if timed {
				c.Users[0].ExpiresAt = time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
				ok16(t, c.Save())
			}
			ok16(t, PlanUnlimitedMigration(c, dir))
			ok16(t, ApplyUnlimitedMigration(dir))
			c, e := config.LoadFrom(c.Path())
			ok16(t, e)
			u := c.Users[0]
			before, lim, e := boundAmount(c.ServerID, u)
			ok16(t, e)
			if before.Confirmed <= 0 || lim != 0 {
				t.Fatal("fixture must retain actual nonzero opening usage", before, lim)
			}
			ok16(t, WriteSubscriptions(c, discardOutput{}))
			for _, suffix := range []string{statusSuffix, ".windows" + statusSuffix} {
				b, e := os.ReadFile(filepath.Join(transport.SubscriptionDir, "sub", u.SubToken+suffix))
				ok16(t, e)
				var v accountStatus
				ok16(t, json.Unmarshal(b, &v))
				if v.Version != 1 || v.Status != statusActive || v.Metering != meteringNone || v.LimitBytes != nil || v.UsedBytes != nil || v.SampledAt != nil || (v.ExpiresAt != nil) != timed || v.Revision < 1 {
					t.Fatalf("released v1 unlimited parser shape violated: %+v", v)
				}
			}
			after, _, e := boundAmount(c.ServerID, u)
			ok16(t, e)
			if before != after {
				t.Fatal("legacy rendering changed ledger", before, after)
			}
			ok16(t, InitializeV2Authority(c))
			snap, e := ResolveV2FromConfig(c.Path(), nil, "")(context.Background(), u.SubToken, "status.json")
			ok16(t, e)
			var v2 StatusV2
			ok16(t, json.Unmarshal(snap.Body, &v2))
			if v2.LimitKind != "unlimited" || v2.Confirmed == nil || *v2.Confirmed != before.Confirmed || v2.Coverage != "partial" {
				t.Fatalf("status2 lost the retained lower bound: %+v", v2)
			}
		})
	}
}
