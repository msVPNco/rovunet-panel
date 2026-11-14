package handlers

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/clientcfg"
	"github.com/msVPNco/rovunet-panel/internal/config"
)

const trustSuffix = ".trust.json"

// Trust is an optional companion resource. The strict v1 feed stays unchanged.
// It is authenticated by the same HTTPS origin and subscription capability;
// it is not an independently signed certificate-rotation authorization.
type subscriptionTrust struct {
	Version int                     `json:"version"`
	Server  string                  `json:"server_id"`
	Account string                  `json:"account_id"`
	Period  string                  `json:"period_id"`
	Items   []subscriptionTrustItem `json:"items"`
}
type subscriptionTrustItem struct {
	Profile     string `json:"profile_id"`
	Transport   string `json:"transport"`
	Domain      string `json:"domain"`
	Certificate string `json:"certificate_pem"`
	SHA256      string `json:"certificate_sha256"`
	NotAfter    string `json:"not_after"`
}

func publicSlipCertificate(path string, now time.Time) (subscriptionTrustItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return subscriptionTrustItem{}, fmt.Errorf("Slipstream public certificate unavailable")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(b) > 65536 {
		return subscriptionTrustItem{}, fmt.Errorf("invalid Slipstream public certificate size")
	}
	block, rest := pem.Decode(b)
	// Accept exactly one public certificate. Never export a mixed key bundle.
	if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 || !bytes.HasPrefix(bytes.TrimSpace(b), []byte("-----BEGIN CERTIFICATE-----")) {
		return subscriptionTrustItem{}, fmt.Errorf("Slipstream trust requires a public-certificate-only PEM")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil || now.Before(c.NotBefore) || !now.Before(c.NotAfter) {
		return subscriptionTrustItem{}, fmt.Errorf("Slipstream public certificate is invalid or outside its validity period")
	}
	h := sha256.Sum256(c.Raw)
	return subscriptionTrustItem{Transport: "slipstream", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})), SHA256: hex.EncodeToString(h[:]), NotAfter: c.NotAfter.UTC().Format(time.RFC3339)}, nil
}

func buildSubscriptionTrust(cfg *config.Config, u config.UserConfig) ([]byte, error) {
	items, err := subscriptionItemsForClient(cfg, u, "")
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, item := range items {
		allowed[item.ID] = true
	}
	v := subscriptionTrust{Version: 1, Server: cfg.ServerID, Account: u.AccountID, Period: u.PeriodID, Items: []subscriptionTrustItem{}}
	for _, t := range cfg.Tunnels {
		if !t.Enabled || t.Transport != config.TransportSlipstream || t.Slipstream == nil {
			continue
		}
		var cert *subscriptionTrustItem
		for _, variant := range clientcfg.Variants(&t) {
			if !allowed[variant.Tag] {
				continue
			}
			if cert == nil {
				c, e := publicSlipCertificate(t.Slipstream.Cert, time.Now())
				if e != nil {
					return nil, e
				}
				cert = &c
			}
			item := *cert
			item.Profile = variant.Tag
			item.Domain = t.Domain
			v.Items = append(v.Items, item)
		}
	}
	return json.MarshalIndent(v, "", "  ")
}
