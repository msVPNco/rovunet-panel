// Package safefile replaces complete files on one filesystem without exposing
// partial contents or following a destination symlink.
package safefile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Write replaces path only after every byte and its permissions are durable.
// The parent must already exist. An error before Rename leaves path intact.
func Write(path string, data []byte, mode os.FileMode) error {
	return Replace(path, mode, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// Replace stages a complete file privately next to its destination. fill must
// finish successfully before the old name is atomically replaced. It never
// falls back to unlinking or truncating an existing destination.
func Replace(path string, mode os.FileMode, fill func(io.Writer) error) (err error) {
	var previous *syscall.Stat_t
	if info, statErr := os.Lstat(path); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("destination must be a regular file")
		}
		previous, _ = info.Sys().(*syscall.Stat_t)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".rovunet-write-*")
	if err != nil {
		return err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	if err = fill(f); err != nil {
		return err
	}
	// os.WriteFile preserved ownership when replacing contents. Keep that
	// contract for files intentionally owned by an unprivileged service.
	if previous != nil {
		current, statErr := f.Stat()
		if statErr != nil {
			return statErr
		}
		if st, ok := current.Sys().(*syscall.Stat_t); ok && (st.Uid != previous.Uid || st.Gid != previous.Gid) {
			if err = f.Chown(int(previous.Uid), int(previous.Gid)); err != nil {
				return err
			}
		}
	}
	if err = f.Chmod(mode.Perm()); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	return SyncDir(dir)
}

// Copy preserves src if reading or staging dst fails. It does not execute src.
func Copy(src, dst string, mode os.FileMode) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return Replace(dst, mode, func(w io.Writer) error {
		_, err := io.Copy(w, f)
		return err
	})
}

// SyncDir makes a completed rename durable across a crash on Linux filesystems.
func SyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
