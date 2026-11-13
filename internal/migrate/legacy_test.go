package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenamePathsLeavesCredentialsAlone(t *testing.T) {
	in := `{"key":"/etc/` + legacy + `/tunnels/a/server.key","dir":"/etc/` + legacy + `","users":[{"name":"` + legacy + `-fan","pass":"` + legacy + `123"}]}`
	got := RenamePaths(in)
	want := `{"key":"/etc/rovunet/tunnels/a/server.key","dir":"/etc/rovunet","users":[{"name":"` + legacy + `-fan","pass":"` + legacy + `123"}]}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestRenameUnit(t *testing.T) {
	unit := "[Unit]\nAfter=" + legacy + "-dnsrouter.service\n[Service]\nUser=" + legacy + "\nGroup=" + legacy + "\nExecStart=/usr/local/bin/" + legacy + " dnsrouter --config /etc/" + legacy + "/config.json\n"
	got := Rename(unit)
	if strings.Contains(got, legacy) {
		t.Fatalf("legacy name left in:\n%s", got)
	}
	for _, want := range []string{"After=rovunet-dnsrouter.service", "User=rovunet\n", "Group=rovunet\n", "ExecStart=/usr/local/bin/rovunet dnsrouter --config /etc/rovunet/config.json"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestNeeded(t *testing.T) {
	root := t.TempDir()
	OldConfigDir, NewConfigDir = filepath.Join(root, "old"), filepath.Join(root, "new")
	if Needed() {
		t.Fatal("nothing installed, yet migration needed")
	}
	if err := os.Mkdir(OldConfigDir, 0750); err != nil {
		t.Fatal(err)
	}
	if !Needed() {
		t.Fatal("old install not detected")
	}
	if err := os.Mkdir(NewConfigDir, 0750); err != nil {
		t.Fatal(err)
	}
	if Needed() {
		t.Fatal("must never migrate over an existing new install")
	}
}

func TestRewriteTextFilesKeepsKeysAndModes(t *testing.T) {
	root := t.TempDir()
	write := func(rel, data string, mode os.FileMode) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("config.json", `{"private_key":"/etc/`+legacy+`/tunnels/a/k"}`, 0600)
	write("warp/wg0.conf", "PostUp = ip rule add uidrange $(id -u "+legacy+"-socks)\n", 0600)
	write("tunnels/a/server.key", "/etc/"+legacy+"/", 0600) // a key is never rewritten

	if err := rewriteTextFiles(root); err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read("config.json"); got != `{"private_key":"/etc/rovunet/tunnels/a/k"}` {
		t.Fatalf("config.json: %s", got)
	}
	if got := read("warp/wg0.conf"); !strings.Contains(got, "rovunet-socks") {
		t.Fatalf("wg0.conf: %s", got)
	}
	if got := read("tunnels/a/server.key"); got != "/etc/"+legacy+"/" {
		t.Fatalf("key file was modified: %s", got)
	}
	if info, _ := os.Stat(filepath.Join(root, "config.json")); info.Mode().Perm() != 0600 {
		t.Fatalf("config.json mode %v", info.Mode().Perm())
	}
}

func TestRenameFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, legacy+"-rules.v4"), []byte("*filter\nCOMMIT\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.conf"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := renameFiles(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "rovunet-rules.v4")); err != nil {
		t.Fatal("not renamed")
	}
	if _, err := os.Stat(filepath.Join(dir, legacy+"-rules.v4")); !os.IsNotExist(err) {
		t.Fatal("old file left")
	}
	if _, err := os.Stat(filepath.Join(dir, "other.conf")); err != nil {
		t.Fatal("unrelated file touched")
	}
}

func TestRepairUnits(t *testing.T) {
	root := t.TempDir()
	SystemdDir, BinDir = filepath.Join(root, "systemd"), filepath.Join(root, "bin")
	for _, d := range []string{SystemdDir, BinDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	old := filepath.Join(BinDir, legacy)
	broken := "[Service]\nExecStart=" + old + " vless serve --listen 127.0.0.1:2053\n"
	ok := "[Service]\nExecStart=/usr/local/bin/dnstt-server -udp :5300\n"
	write := func(n, s string) {
		if err := os.WriteFile(filepath.Join(SystemdDir, n), []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("rovunet-vless.service", broken)
	write("rovunet-dnstt.service", ok)
	write("other.service", broken) // not the panel's: never touched

	// While the old command exists, units using it still start: no change.
	if err := os.WriteFile(old, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	if fixed, err := RepairUnits(); err != nil || len(fixed) != 0 {
		t.Fatalf("fixed %v while the old command exists (%v)", fixed, err)
	}
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	fixed, err := RepairUnits()
	if err != nil || len(fixed) != 1 || fixed[0] != "rovunet-vless" {
		t.Fatalf("fixed %v, %v", fixed, err)
	}
	b, _ := os.ReadFile(filepath.Join(SystemdDir, "rovunet-vless.service"))
	if want := "ExecStart=" + filepath.Join(BinDir, "rovunet") + " vless serve"; !strings.Contains(string(b), want) {
		t.Fatalf("unit not repaired:\n%s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(SystemdDir, "other.service")); string(b) != broken {
		t.Fatal("a unit that is not the panel's was changed")
	}
}
