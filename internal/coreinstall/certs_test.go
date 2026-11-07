package coreinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCore(t *testing.T, name, certFile string) {
	t.Helper()
	dir := filepath.Join(Root, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta"), []byte("protocol=hysteria2\nport=8443\nproto=udp\nname="+name+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := `{"inbounds":[{"streamSettings":{"tlsSettings":{"certificates":[{"certificateFile":"` + certFile + `","keyFile":"k"}]}}}]}`
	if err := os.WriteFile(filepath.Join(dir, "server.json"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRestartIfCertChanged(t *testing.T) {
	Root = t.TempDir()
	cert := filepath.Join(t.TempDir(), "c.crt")
	if err := os.WriteFile(cert, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	writeCore(t, "hy2", cert)
	writeCore(t, "plain", "")
	var restarted []string
	old := restartCore
	restartCore = func(unit string) error { restarted = append(restarted, unit); return nil }
	defer func() { restartCore = old }()

	if got, err := RestartIfCertChanged(); err != nil || len(got) != 0 {
		t.Fatalf("first run must only record: %v %v", got, err)
	}
	if got, _ := RestartIfCertChanged(); len(got) != 0 {
		t.Fatalf("unchanged certificate restarted %v", got)
	}
	if err := os.WriteFile(cert, []byte("renewed"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := RestartIfCertChanged()
	if err != nil || len(got) != 1 || got[0] != "hy2" || len(restarted) != 1 || restarted[0] != "rovunet-core-hy2" {
		t.Fatalf("renewed certificate: got %v %v, restarted %v", got, err, restarted)
	}
	if got, _ := RestartIfCertChanged(); len(got) != 0 {
		t.Fatalf("restarted twice for one renewal: %v", got)
	}
}
