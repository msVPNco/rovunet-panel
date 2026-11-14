package handlers

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
)

func TestInstallUsernameDoesNotAdoptExistingLinuxAccount(t *testing.T) {
	c := &config.Config{Users: []config.UserConfig{{Username: "existing-panel"}}}
	exists := func(name string) bool { return name == "mohsen" || name == "root" }
	for _, name := range []string{"mohsen", "root", "existing-panel", "-unsafe"} {
		if checkInstallUsername(c, name, exists) == nil {
			t.Errorf("accepted occupied/unsafe name %q", name)
		}
	}
	if err := checkInstallUsername(c, "mohsen-vpn", exists); err != nil {
		t.Fatal(err)
	}
}

func TestUninstallArchivesWALAndKeysWithoutFollowingSymlinks(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ledger.sqlite", "ledger.sqlite-wal", "private-key"} {
		if err := os.WriteFile(filepath.Join(data, name), []byte("synthetic-"+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(root, "not-owned")
	if err := os.WriteFile(outside, []byte("DO-NOT-COPY"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(data, "link")); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(data, "runtime.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	dir, err := archiveUninstallData(root, []string{data})
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(dir)
	if info.Mode().Perm() != 0700 {
		t.Fatal("archive directory is not private")
	}
	f, err := os.Open(filepath.Join(dir, "private-data.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, _ = f.Stat()
	if info.Mode().Perm() != 0600 {
		t.Fatal("archive file is not private")
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tarReader := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		h, e := tarReader.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(tarReader)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(body), "DO-NOT-COPY") {
			t.Fatal("followed external symlink")
		}
		seen[filepath.Base(h.Name)] = true
	}
	for _, name := range []string{"ledger.sqlite", "ledger.sqlite-wal", "private-key", "link"} {
		if !seen[name] {
			t.Errorf("missing archived %s", name)
		}
	}
	if seen["runtime.sock"] {
		t.Fatal("ephemeral runtime socket was archived")
	}
	if _, err := os.Stat(filepath.Join(data, "ledger.sqlite")); err != nil {
		t.Fatal("archiving deleted live data")
	}
}

func TestUninstallIncludesNewEnginesAndBootstrapReceipts(t *testing.T) {
	dir := t.TempDir()
	wanted := []string{"rovunet", "rovunet.bak", "rovunet-amneziawg-go", "rovunet-awg", "rovunet-awg-provenance.json", "rovunet-sing-box", "rovunet-mita", ".rovunet-install-state.json", ".rovunet-update-state.json", "xray.before-reviewed", "xray.provenance.json"}
	for _, name := range append(wanted, "unrelated-app", "other-panel.bak") {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := uninstallBinaryNames(entries)
	if len(got) != len(wanted) {
		t.Fatalf("unexpected uninstall targets %v", got)
	}
	for _, name := range wanted {
		found := false
		for _, v := range got {
			found = found || name == v
		}
		if !found {
			t.Errorf("missed %s", name)
		}
	}
}

func TestUninstallSharedBinRequiresExactOwnershipReceipt(t *testing.T) {
	dir := t.TempDir()
	owned := map[string]string{}
	for name, body := range map[string]string{"xray": "third party engine", "xray.customer-backup": "customer backup", "rovunet-awg": "panel engine", "rovunet-awg.bak": "owned recovery", "caddy-naive": "changed after install", ".rovunet-install-state.json": "unknown receipt"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if name == "rovunet-awg" || name == "rovunet-awg.bak" || name == "caddy-naive" {
			hash, err := binary.FileSHA256(path)
			if err != nil {
				t.Fatal(err)
			}
			owned[path] = hash
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "caddy-naive"), []byte("operator replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	remove, keep := verifiedUninstallFiles(dir, entries, owned)
	if len(remove) != 2 || len(keep) != 4 {
		t.Fatalf("wrong removal plan: remove=%v keep=%v", remove, keep)
	}
	for _, name := range remove {
		if name != "rovunet-awg" && name != "rovunet-awg.bak" {
			t.Errorf("third-party/changed object selected: %s", name)
		}
	}
}
