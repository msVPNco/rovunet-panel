package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func trustFixture25(t *testing.T) (*config.Config, string, []byte) {
	t.Helper()
	c := subTestConfig()
	c.Users = c.Users[:1]
	k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ok16(t, e)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, &k.PublicKey, k)
	ok16(t, e)
	b := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	p := filepath.Join(t.TempDir(), "public.pem")
	ok16(t, os.WriteFile(p, b, 0600))
	c.Tunnels = append(c.Tunnels, config.TunnelConfig{Tag: "slipstream-socks", Transport: config.TransportSlipstream, Backend: config.BackendSOCKS, Domain: "slip.example.test", Enabled: true, Slipstream: &config.SlipstreamConfig{Cert: p, Key: "DO-NOT-READ-PRIVATE-KEY"}})
	return c, p, b
}

func TestTrustPublicOnlyAndStableFeed(t *testing.T) {
	c, p, cert := trustFixture25(t)
	u := c.Users[0]
	before, e := buildFeed(c, u)
	ok16(t, e)
	b, e := buildSubscriptionTrust(c, u)
	ok16(t, e)
	if path := os.Getenv("ROVUNET_TRUST_FIXTURE"); path != "" {
		ok16(t, os.WriteFile(path, b, 0644))
	}
	var v subscriptionTrust
	ok16(t, json.Unmarshal(b, &v))
	if len(v.Items) != 1 || v.Items[0].Profile != "slipstream-socks" || v.Items[0].Certificate != string(cert) || len(v.Items[0].SHA256) != 64 || strings.Contains(string(b), "DO-NOT-READ") {
		t.Fatalf("wrong public trust document: %+v", v)
	}
	after, e := buildFeed(c, u)
	ok16(t, e)
	if string(before) != string(after) {
		t.Fatal("legacy feed changed")
	}
	for _, bad := range [][]byte{[]byte("garbage"), append(append([]byte{}, cert...), []byte("-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\n")...), append([]byte("private-prefix\n"), cert...)} {
		ok16(t, os.WriteFile(p, bad, 0600))
		if _, e := buildSubscriptionTrust(c, u); e == nil {
			t.Fatal("invalid or mixed secret certificate published")
		}
	}
}

func TestTrustRetiredWithOldTokenAndAccount(t *testing.T) {
	old := transport.SubscriptionDir
	transport.SubscriptionDir = filepath.Join(t.TempDir(), "subs")
	defer func() { transport.SubscriptionDir = old }()
	c, _, _ := trustFixture25(t)
	token := c.Users[0].SubToken
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	path := filepath.Join(publicDir(), token+trustSuffix)
	if _, e := os.Stat(path); e != nil {
		t.Fatal(e)
	}
	c.Users[0].SubToken = strings.Repeat("d4", 16)
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("old trust capability survived")
	}
	current := filepath.Join(publicDir(), c.Users[0].SubToken+trustSuffix)
	c.Users = nil
	ok16(t, WriteSubscriptions(c, discardOutput{}))
	if _, e := os.Stat(current); !os.IsNotExist(e) {
		t.Fatal("deleted account trust survived")
	}
}
