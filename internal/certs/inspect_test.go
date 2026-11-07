package certs

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDescribe(t *testing.T) {
	dir := t.TempDir()
	cert, key := filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	if err := GenerateSelfSigned(cert, key, "t.example.com"); err != nil {
		t.Fatal(err)
	}
	info, err := Inspect(cert)
	if err != nil || !info.SelfSigned {
		t.Fatalf("%+v %v", info, err)
	}
	now := time.Now()
	if d := Describe(cert, now); !strings.Contains(d, "self-signed") || strings.Contains(d, "EXPIR") {
		t.Fatalf("fresh cert: %q", d)
	}
	if d := Describe(cert, info.NotAfter.Add(-24*time.Hour)); !strings.HasPrefix(strings.SplitN(d, ", ", 3)[2], "EXPIRING SOON") {
		t.Fatalf("near expiry: %q", d)
	}
	if d := Describe(cert, info.NotAfter.Add(time.Hour)); !strings.Contains(d, "EXPIRED") {
		t.Fatalf("expired: %q", d)
	}
	if d := Describe(filepath.Join(dir, "missing.pem"), now); !strings.HasPrefix(d, "UNREADABLE") {
		t.Fatalf("missing: %q", d)
	}
	if Describe(key, now) == "" || !strings.HasPrefix(Describe(key, now), "UNREADABLE") {
		t.Fatal("a key file must not be reported as a certificate")
	}
}
