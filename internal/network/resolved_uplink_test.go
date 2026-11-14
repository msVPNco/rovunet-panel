package network

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestResolvedUplinkPreservesAdministratorResolvers(t *testing.T) {
	for _, kind := range []string{"stub", "relative-stub", "static", "other-link", "missing-uplink"} {
		t.Run(kind, func(t *testing.T) {
			d := t.TempDir()
			path := filepath.Join(d, "resolv.conf")
			stub := filepath.Join(d, "stub-resolv.conf")
			uplink := filepath.Join(d, "uplink-resolv.conf")
			if kind != "missing-uplink" {
				if err := os.WriteFile(uplink, []byte("nameserver 192.0.2.53\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			switch kind {
			case "static":
				err = os.WriteFile(path, []byte("administrator resolver\n"), 0600)
			case "other-link":
				err = os.Symlink("administrator.conf", path)
			case "relative-stub":
				err = os.Symlink("stub-resolv.conf", path)
			default:
				err = os.Symlink(stub, path)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = useResolvedUplink(path, stub, uplink)
			if kind == "missing-uplink" {
				if err == nil {
					t.Fatal("missing uplink accepted")
				}
				got, _ := os.Readlink(path)
				if got != stub {
					t.Fatal("failed selection changed resolver")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if kind == "static" {
				got, _ := os.ReadFile(path)
				if string(got) != "administrator resolver\n" {
					t.Fatal("static resolver overwritten")
				}
				return
			}
			got, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "other-link" {
				if got != "administrator.conf" {
					t.Fatal("administrator link overwritten")
				}
			} else if got != uplink {
				t.Fatal("stub link did not select uplink")
			}
		})
	}
}

func TestResolvedDropinPermissionsAndSymlinkBoundary(t *testing.T) {
	d := t.TempDir()
	dir := filepath.Join(d, "dropins")
	old := syscall.Umask(0077)
	defer syscall.Umask(old)
	if err := writeResolvedStubDropin(dir); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{dir: 0755, filepath.Join(dir, "rovunet-no-stub.conf"): 0644} {
		i, err := os.Stat(p)
		if err != nil || i.Mode().Perm() != want {
			t.Fatal(p, i, err)
		}
	}
	sentinel := filepath.Join(d, "private")
	if err := os.WriteFile(sentinel, []byte("private sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(d, "linked-dir")
	if err := os.Symlink(d, linkDir); err != nil {
		t.Fatal(err)
	}
	if writeResolvedStubDropin(linkDir) == nil {
		t.Fatal("directory symlink accepted")
	}
	p := filepath.Join(dir, "rovunet-no-stub.conf")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, p); err != nil {
		t.Fatal(err)
	}
	if writeResolvedStubDropin(dir) == nil {
		t.Fatal("drop-in symlink accepted")
	}
	raw, _ := os.ReadFile(sentinel)
	if string(raw) != "private sentinel" {
		t.Fatal("private target overwritten")
	}
}
