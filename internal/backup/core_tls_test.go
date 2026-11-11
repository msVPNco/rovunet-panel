package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCoreTLSBackupRestoresOutsideCertificateAndKey(t *testing.T) {
	source := t.TempDir()
	cert := filepath.Join(source, "public.crt")
	key := filepath.Join(source, "private.key")
	if err := os.WriteFile(cert, []byte("fixture-certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("fixture-private-key"), 0600); err != nil {
		t.Fatal(err)
	}
	server, _ := json.Marshal(map[string]any{"inbounds": []any{map[string]any{"streamSettings": map[string]any{"tlsSettings": map[string]any{"certificates": []any{map[string]any{"certificateFile": cert, "keyFile": key}}}}}}})
	options, _ := json.Marshal(map[string]any{"protocol": "trojan", "cert": cert, "key": key})
	files := map[string]File{"server.json": {Data: server}, "options.json": {Data: options}}
	if err := captureCoreTLS(files, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if string(files["tls-key.pem"].Data) != "fixture-private-key" {
		t.Fatal("external private key missing from private backup")
	}
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		if err := restoreCoreTLS(files, dir); err != nil {
			t.Fatal(err)
		}
		var o map[string]any
		_ = json.Unmarshal(files["options.json"].Data, &o)
		if o["cert"] != filepath.Join(dir, "tls-cert.pem") || o["key"] != filepath.Join(dir, "tls-key.pem") {
			t.Fatal("restored paths did not follow their new core directory")
		}
	}
}
