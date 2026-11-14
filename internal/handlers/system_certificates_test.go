package handlers

import (
	"github.com/msVPNco/rovunet-panel/internal/certs"
	"path/filepath"
	"testing"
	"time"
)

func TestCertificatePairRecoveryValidation(t *testing.T) {
	d := t.TempDir()
	crt, key := filepath.Join(d, "cert.pem"), filepath.Join(d, "key.pem")
	ok16(t, certs.GenerateSelfSigned(crt, key, "tls.example.org"))
	if _, e := certificatePair(crt, key, "", time.Now()); e != nil {
		t.Fatal("pin-only legacy certificate rejected", e)
	}
	if _, e := certificatePair(crt, key, "tls.example.org", time.Now()); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name, host string
		now        time.Time
	}{{"expired", "tls.example.org", time.Now().AddDate(11, 0, 0)}, {"future", "tls.example.org", time.Now().Add(-time.Hour)}, {"wrong-host", "other.example.org", time.Now()}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, e := certificatePair(crt, key, tc.host, tc.now); e == nil {
				t.Fatal("invalid certificate accepted")
			}
		})
	}
	otherCert, otherKey := filepath.Join(d, "other.pem"), filepath.Join(d, "other.key")
	ok16(t, certs.GenerateSelfSigned(otherCert, otherKey, "tls.example.org"))
	if _, e := certificatePair(crt, otherKey, "tls.example.org", time.Now()); e == nil {
		t.Fatal("mismatched key accepted")
	}
}

func TestDiagnosticsRespectDisabledAndPausedCores(t *testing.T) {
	for _, tc := range []struct {
		data   string
		active bool
	}{{`{}`, true}, {`{"disabled":true}`, false}, {`{"paused":true}`, false}} {
		active, _, e := diagnosticCoreExpectedActive([]byte(tc.data))
		ok16(t, e)
		if active != tc.active {
			t.Fatal(tc)
		}
	}
	if _, _, e := diagnosticCoreExpectedActive([]byte(`bad`)); e == nil {
		t.Fatal("corrupt state ignored")
	}
}
