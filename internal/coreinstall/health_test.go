package coreinstall

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCertExpiry(t *testing.T) {
	Root = t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	notAfter := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "t"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
	}, &x509.Certificate{SerialNumber: big.NewInt(1)}, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := filepath.Join(t.TempDir(), "c.crt")
	if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	writeCore(t, "hy2", cert)
	writeCore(t, "plain", "")

	got, ok, err := CertExpiry("hy2")
	if err != nil || !ok || !got.Equal(notAfter) {
		t.Fatalf("hy2: %v %v %v, want %v", got, ok, err, notAfter)
	}
	if _, ok, _ := CertExpiry("plain"); ok {
		t.Fatal("a core without a certificate must report none")
	}
	writeCore(t, "broken", filepath.Join(t.TempDir(), "missing.crt"))
	if _, ok, err := CertExpiry("broken"); !ok || err == nil {
		t.Fatalf("missing certificate file must be an error, got ok=%v err=%v", ok, err)
	}
}
