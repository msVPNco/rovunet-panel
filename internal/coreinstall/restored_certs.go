package coreinstall

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// A restored core has a private certificate copy so its backup works without
// /root's Caddy storage. When the original ACME location becomes available,
// keep that copy in step with valid, matching renewals.
func refreshRestoredCerts(name string) error {
	dir := filepath.Join(Root, name)
	b, err := os.ReadFile(filepath.Join(dir, "tls-sources.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	sources := map[string]string{}
	if err := json.Unmarshal(b, &sources); err != nil {
		return err
	}
	cert, key := sources["tls-cert.pem"], sources["tls-key.pem"]
	if cert == "" || key == "" {
		return nil
	}
	if _, err := os.Stat(cert); os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(key); os.IsNotExist(err) {
		return nil
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return fmt.Errorf("core %s: renewed certificate/key pair is invalid", name)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return err
	}
	if time.Now().Before(leaf.NotBefore) || time.Now().After(leaf.NotAfter) {
		return fmt.Errorf("core %s: renewed certificate is outside its validity period", name)
	}
	if o, _, managed, err := loadUserCore(name); err != nil {
		return err
	} else if managed {
		if err := leaf.VerifyHostname(o.SNI); err != nil {
			return fmt.Errorf("core %s: renewed certificate does not match its SNI", name)
		}
	}
	for source, target := range map[string]string{cert: "tls-cert.pem", key: "tls-key.pem"} {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, target)
		if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
			return err
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			return err
		}
	}
	return nil
}
