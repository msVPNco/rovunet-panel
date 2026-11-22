package protocols

import "testing"

func TestShadowsocksVariantsNeverBypassTrafficQuotas(t *testing.T) {
	for _, protocol := range []string{"ss2022", "ss2022-aes128", "ss-aead-chacha20"} {
		if !IsShadowsocks(protocol) || !Metered(protocol) || QuotaEligible(protocol) {
			t.Fatalf("incorrect accounting policy for %s", protocol)
		}
		for _, timed := range []bool{false, true} {
			if !Allowed(protocol, false, timed) || Allowed(protocol, true, timed) {
				t.Fatalf("incorrect eligibility for %s, timed=%v", protocol, timed)
			}
		}
	}
}
