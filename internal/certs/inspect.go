package certs

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

// Info summarizes the first certificate in a PEM file.
type Info struct {
	Names      []string
	NotAfter   time.Time
	SelfSigned bool
}

// Inspect reads the first certificate in the PEM file at path.
func Inspect(path string) (Info, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Info{}, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return Info{}, fmt.Errorf("no PEM certificate in %s", path)
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return Info{}, err
	}
	names := c.DNSNames
	if len(names) == 0 && c.Subject.CommonName != "" {
		names = []string{c.Subject.CommonName}
	}
	// CheckSignatureFrom would refuse a non-CA leaf, so verify the raw signature.
	self := c.Issuer.String() == c.Subject.String() &&
		c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature) == nil
	return Info{Names: names, NotAfter: c.NotAfter, SelfSigned: self}, nil
}

// Describe is a one-line status for a certificate file, flagging expired
// and soon-to-expire certificates so they're noticed before clients fail.
func Describe(path string, now time.Time) string {
	if path == "" {
		return "none"
	}
	info, err := Inspect(path)
	if err != nil {
		return "UNREADABLE: " + err.Error()
	}
	kind := "CA-issued"
	if info.SelfSigned {
		kind = "self-signed"
	}
	left := info.NotAfter.Sub(now)
	state := fmt.Sprintf("expires %s (%d days)", info.NotAfter.Format("2006-01-02"), int(left.Hours()/24))
	switch {
	case left <= 0:
		state = "EXPIRED " + info.NotAfter.Format("2006-01-02")
	case left < 14*24*time.Hour:
		state = "EXPIRING SOON: " + state
	}
	return fmt.Sprintf("%s, %s, %s", strings.Join(info.Names, ","), kind, state)
}
