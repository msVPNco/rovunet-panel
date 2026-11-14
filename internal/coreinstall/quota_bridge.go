package coreinstall

import (
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/keys"
	"time"
)

func bridgeEligible(o coreplan.Options, u config.UserConfig) bool {
	b := o.QuotaBridge
	return b != nil && coreplan.BridgeProtocol(o.Protocol) && u.QuotaGB > 0 && u.SQLiteMeter != nil && b.Directory == u.SQLiteMeter.Directory && b.Store == u.SQLiteMeter.Store && !u.Expired(time.Now())
}

func UsesQuotaBridge(name string) bool {
	o, _, managed, err := loadUserCore(name)
	return err == nil && managed && o.QuotaBridge != nil && coreplan.BridgeProtocol(o.Protocol)
}
func bridgeUsers(o coreplan.Options, users, eligible []config.UserConfig) ([]config.UserConfig, map[string]string, error) {
	b := o.QuotaBridge
	if b == nil || !coreplan.BridgeProtocol(o.Protocol) {
		return nil, nil, fmt.Errorf("unsupported quota bridge")
	}
	secret, e := keys.LoadVLESSSecret(b.SecretFile)
	if e != nil {
		return nil, nil, e
	}
	ids := map[string]string{}
	for _, u := range users {
		if !bridgeEligible(o, u) {
			continue
		}
		id := keys.VLESSUserID(secret, u.Username, u.Password)
		ids[u.Username] = fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
		eligible = append(eligible, u)
	}
	return eligible, ids, nil
}
