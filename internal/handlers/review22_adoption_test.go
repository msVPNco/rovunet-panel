package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/recoveryauth"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyRevisionAdoption22(t *testing.T) {
	for _, phase := range []string{"adopt-binding", "adopt-files", "adopt-ready"} {
		t.Run(phase, func(t *testing.T) {
			c, v := revisionFixture22(t)
			root, _ := authorityPaths(c)
			data, e := os.ReadFile(filepath.Join(root, "records.json"))
			ok16(t, e)
			var existing revisionRegistry
			ok16(t, json.Unmarshal(data, &existing))
			legacy, _ := json.Marshal(existing.Records)
			ok16(t, os.WriteFile(filepath.Join(root, "records.json"), legacy, 0600))
			ok16(t, os.WriteFile(c.Path()+".status-v2.anchor", []byte("private-v2-revisions-v1"), 0600))
			c.V2Authority = nil
			ok16(t, c.Save())
			raw, e := os.ReadFile(c.Path())
			ok16(t, e)
			pub, key, e := ed25519.GenerateKey(rand.Reader)
			ok16(t, e)
			plan := revisionAdoption{1, c.ServerID, config.NewID(), digest(raw), digest(legacy), digest([]byte("private-v2-revisions-v1"))}
			envelope, e := recoveryauth.Sign(plan, key)
			ok16(t, e)
			pinned := hex.EncodeToString(pub)
			if InitializeV2Authority(c) == nil || PrepareV2Authority(c) == nil {
				t.Fatal("legacy state silently initialized")
			}
			revisionBarrier = func(s string) error {
				if s == phase {
					return os.ErrPermission
				}
				return nil
			}
			if AdoptLegacyV2Authority(c, envelope, pinned) == nil {
				t.Fatal("fault not injected")
			}
			revisionBarrier = nil
			t.Cleanup(func() { revisionBarrier = nil })
			ok16(t, c.Reload())
			ok16(t, AdoptLegacyV2Authority(c, envelope, pinned))
			ok16(t, ValidateV2Authority(c))
			lock, e := revisionLock(c, false)
			ok16(t, e)
			r, _, e := readRevision(c)
			lock.Close()
			ok16(t, e)
			for _, record := range r.Records {
				if record.Revision < v.Revision || record.Confirmed == nil || *record.Confirmed != 41 {
					t.Fatal("adoption lost prior high-water")
				}
			}
			if c.V2Authority.ID != plan.Authority {
				t.Fatal("adoption identity changed")
			}
		})
	}
}
