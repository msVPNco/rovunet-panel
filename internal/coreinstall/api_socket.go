package coreinstall

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// A hard power cut leaves pathname sockets on persistent /etc. Never unlink
// a live endpoint, another path, a symlink, or a file mistaken for a socket.
func prepareAPISocket(name, socket string) error {
	if socket == "" {
		return nil
	}
	dir := filepath.Join(Root, name)
	expected := filepath.Join(dir, "api.sock")
	if socket != expected {
		return fmt.Errorf("core API socket is outside its owned directory")
	}
	parent, err := os.Lstat(dir)
	if err != nil || !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 || parent.Mode().Perm()&0022 != 0 {
		return fmt.Errorf("core API socket requires a private regular parent directory")
	}
	before, err := os.Lstat(socket)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if before.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("core API endpoint is not a regular Unix socket")
	}
	conn, dialErr := net.DialTimeout("unix", socket, 150*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		return fmt.Errorf("core API socket already has an active listener; existing endpoint preserved")
	}
	if errors.Is(dialErr, syscall.ENOENT) {
		return nil
	}
	if !errors.Is(dialErr, syscall.ECONNREFUSED) {
		return fmt.Errorf("core API socket liveness is uncertain; existing endpoint preserved")
	}
	after, err := os.Lstat(socket)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if after.Mode()&os.ModeSocket == 0 || !os.SameFile(before, after) {
		return fmt.Errorf("core API endpoint changed during stale-socket inspection")
	}
	return os.Remove(socket)
}
