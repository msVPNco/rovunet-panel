package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
)

var ErrNoAccountEvidence = errors.New("no recorded usage evidence for account")

type MigrationSnapshot struct {
	Total       int64
	Fingerprint string
	Partial     bool
	Sources     int
}

func RecordedSnapshot(user string) (MigrationSnapshot, error) {
	var out MigrationSnapshot
	entries, e := os.ReadDir(Dir)
	if os.IsNotExist(e) {
		return out, ErrNoAccountEvidence
	}
	if e != nil {
		return out, e
	}
	h := sha256.New()
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), ".source")) {
			continue
		}
		p := filepath.Join(Dir, entry.Name())
		info, e := os.Lstat(p)
		if e != nil {
			return out, e
		}
		if !info.Mode().IsRegular() {
			return out, errors.New("nonregular usage source")
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return out, e
		}
		var m map[string]int64
		if strings.HasSuffix(entry.Name(), ".source") {
			var s AbsoluteSource
			if e = json.Unmarshal(b, &s); e != nil || s.Epoch == "" || s.Last == nil || s.Totals == nil {
				return out, errors.New("invalid absolute source")
			}
			for _, n := range s.Last {
				if n < 0 {
					return out, errors.New("negative source watermark")
				}
			}
			m = s.Totals
			out.Partial = out.Partial || s.Incomplete
		} else {
			if e = json.Unmarshal(b, &m); e != nil || m == nil {
				return out, errors.New("invalid usage source")
			}
		}
		for _, n := range m {
			if n < 0 {
				return out, errors.New("negative usage source")
			}
		}
		if n, ok := m[user]; ok {
			if n > math.MaxInt64-out.Total {
				return out, errors.New("usage overflow")
			}
			out.Total += n
			out.Sources++
		}
		sum := sha256.Sum256(b)
		h.Write([]byte(entry.Name()))
		h.Write([]byte{0})
		h.Write(sum[:])
	}
	if out.Sources == 0 {
		return out, ErrNoAccountEvidence
	}
	out.Fingerprint = hex.EncodeToString(h.Sum(nil))
	// Unknown protocols and unsampled intervals remain outside the lower bound.
	out.Partial = true
	return out, nil
}
