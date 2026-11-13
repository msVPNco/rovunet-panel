package quota

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// ResumeBootstrap never creates a file or schema. A caller must replay its
// authenticated original manifest idempotently and explicitly finish it.
func ResumeBootstrap(ctx context.Context, dir, server, id string) (*Store, error) {
	b, e := os.ReadFile(filepath.Join(dir, "bootstrap.pending"))
	if e != nil {
		return nil, e
	}
	if string(b) != id {
		return nil, ErrConflict
	}
	s, e := connect(ctx, dir, server, id)
	if e != nil {
		return nil, e
	}
	var ds, di string
	var v int
	e = s.db.QueryRowContext(ctx, "SELECT server_id,store_id,schema_version FROM store_meta WHERE singleton=1").Scan(&ds, &di, &v)
	if e != nil || ds != server || di != id || v != 1 {
		s.Close()
		return nil, ErrConflict
	}
	return s, nil
}
func secureAncestors(path string) error {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		st, e := os.Lstat(dir)
		if e != nil {
			return e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("invalid storage ancestor")
		}
		uid := st.Sys().(*syscall.Stat_t).Uid
		if uid != 0 && uid != uint32(os.Geteuid()) {
			return errors.New("foreign storage ancestor")
		}
		if st.Mode().Perm()&0022 != 0 && !(uid == 0 && st.Mode()&os.ModeSticky != 0) {
			return errors.New("writable storage ancestor")
		}
		if dir == filepath.Dir(dir) {
			return nil
		}
	}
}
