package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/recoveryauth"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"os"
	"path/filepath"
	"syscall"
)

type revisionRegistry struct {
	Version           int
	Authority, Server string
	Records           map[string]statusRecord
}
type revisionAnchor struct {
	Version                 int
	Authority, Server, Hash string
}
type revisionIntent struct {
	Version                              int
	Authority, Server, Before, AfterHash string
	After                                []byte
}

var revisionBarrier func(string) error

func revisionStep(s string) error {
	if revisionBarrier != nil {
		return revisionBarrier(s)
	}
	return nil
}
func authorityPaths(c *config.Config) (string, string) {
	r := c.Path() + ".status-v2"
	return r, filepath.Join(r, "anchor.json")
}
func checkAuthorityBinding(c *config.Config, state string) error {
	b := c.V2Authority
	if b == nil || b.State != state || b.Server != c.ServerID || !validPublicationID(b.ID, b.Server) {
		return errors.New("revision authority binding missing, legacy or not ready")
	}
	return nil
}
func privateRevisionFile(path string) ([]byte, error) {
	st, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("invalid private revision file")
	}
	return os.ReadFile(path)
}
func revisionLock(c *config.Config, exclusive bool) (*os.File, error) {
	root, _ := authorityPaths(c)
	st, e := os.Lstat(root)
	if e != nil {
		return nil, e
	}
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 || st.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid private revision directory")
	}
	f, e := os.OpenFile(filepath.Join(root, "write.lock"), os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	op := syscall.LOCK_SH
	if exclusive {
		op = syscall.LOCK_EX
	}
	if e = syscall.Flock(int(f.Fd()), op); e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func decodeRegistry(c *config.Config, raw []byte) (revisionRegistry, error) {
	var r revisionRegistry
	if e := json.Unmarshal(raw, &r); e != nil {
		return r, e
	}
	if c.V2Authority == nil || r.Version != 2 || r.Authority != c.V2Authority.ID || r.Server != c.ServerID || r.Records == nil {
		return r, errors.New("revision registry identity/version missing")
	}
	for k, v := range r.Records {
		if !validPublicationID(v.Server, v.Account, v.Period) || v.Server != c.ServerID || v.Revision <= 0 || v.Semantic == "" || (v.Confirmed != nil && *v.Confirmed < 0) || k != digest([]byte(v.Server+"/"+v.Account+"/"+v.Period)) {
			return r, errors.New("incomplete revision registry")
		}
	}
	return r, nil
}
func readRevision(c *config.Config) (revisionRegistry, []byte, error) {
	var r revisionRegistry
	if e := checkAuthorityBinding(c, "ready"); e != nil {
		return r, nil, e
	}
	root, anchor := authorityPaths(c)
	identity, e := privateRevisionFile(c.Path() + ".status-v2.anchor")
	if e != nil {
		return r, nil, e
	}
	if string(identity) != "private-v2-authority:"+c.V2Authority.ID {
		return r, nil, errors.New("revision completion identity mismatch")
	}
	data, e := privateRevisionFile(filepath.Join(root, "records.json"))
	if e != nil {
		return r, nil, e
	}
	a, e := privateRevisionFile(anchor)
	if e != nil {
		return r, nil, e
	}
	var head revisionAnchor
	if json.Unmarshal(a, &head) != nil || head.Version != 2 || head.Authority != c.V2Authority.ID || head.Server != c.ServerID || head.Hash != digest(data) {
		return r, nil, errors.New("revision high-water mismatch")
	}
	if _, e = os.Lstat(filepath.Join(root, "transaction.json")); e == nil || !os.IsNotExist(e) {
		return r, nil, errors.New("revision commit pending; explicit recovery required")
	}
	r, e = decodeRegistry(c, data)
	return r, data, e
}
func ValidateV2Authority(c *config.Config) error {
	if e := checkAuthorityBinding(c, "ready"); e != nil {
		return e
	}
	lock, e := revisionLock(c, false)
	if e != nil {
		return e
	}
	defer lock.Close()
	_, _, e = readRevision(c)
	return e
}

// InitializeV2Authority is explicit, never called by refresh/post-update/serve.
func InitializeV2Authority(c *config.Config) error {
	return withPublicationLock(func() error {
		if e := c.RefreshIfChanged(); e != nil {
			return e
		}
		root, anchor := authorityPaths(c)
		if c.V2Authority != nil && c.V2Authority.State == "ready" {
			return ValidateV2Authority(c)
		}
		if c.V2Authority == nil {
			if !validPublicationID(c.ServerID) {
				return errors.New("persist a valid logical server identity before authority initialization")
			}
			if c.SubscriptionV2 {
				return errors.New("legacy enabled installation requires authenticated adoption")
			}
			for _, p := range []string{root, c.Path() + ".status-v2.anchor", c.Path() + ".status-v2.adoption"} {
				if _, e := os.Lstat(p); e == nil || !os.IsNotExist(e) {
					return errors.New("existing revision evidence requires authenticated adoption")
				}
			}
			c.V2Authority = &config.RevisionAuthority{ID: config.NewID(), Server: c.ServerID, State: "initializing"}
			if e := c.Save(); e != nil {
				return e
			}
			if e := revisionStep("init-binding"); e != nil {
				return e
			}
		}
		if e := checkAuthorityBinding(c, "initializing"); e != nil {
			return e
		}
		if c.SubscriptionV2 {
			return errors.New("initializing authority must remain disabled")
		}
		if e := os.Mkdir(root, 0700); e != nil && !os.IsExist(e) {
			return e
		}
		lock, e := os.OpenFile(filepath.Join(root, "write.lock"), os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
		if e != nil {
			return e
		}
		defer lock.Close()
		if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
			return e
		}
		r := revisionRegistry{Version: 2, Authority: c.V2Authority.ID, Server: c.ServerID, Records: map[string]statusRecord{}}
		raw, _ := json.Marshal(r)
		head, _ := json.Marshal(revisionAnchor{2, r.Authority, r.Server, digest(raw)})
		for _, item := range []struct {
			path string
			raw  []byte
		}{{filepath.Join(root, "records.json"), raw}, {anchor, head}, {c.Path() + ".status-v2.anchor", []byte("private-v2-authority:" + r.Authority)}} {
			before, e := os.ReadFile(item.path)
			if e == nil {
				if string(before) != string(item.raw) {
					return errors.New("initialization found changed authority; do not overwrite")
				}
			} else if os.IsNotExist(e) {
				if e = safefile.Write(item.path, item.raw, 0600); e != nil {
					return e
				}
			} else {
				return e
			}
		}
		if e = revisionStep("init-files"); e != nil {
			return e
		}
		c.V2Authority = &config.RevisionAuthority{ID: r.Authority, Server: r.Server, State: "ready"}
		if e = c.Save(); e != nil {
			return e
		}
		return revisionStep("init-ready")
	})
}
func persistRevision(c *config.Config, before []byte, r revisionRegistry) error {
	root, anchor := authorityPaths(c)
	after, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if string(before) == string(after) {
		return nil
	}
	intent := revisionIntent{2, r.Authority, r.Server, digest(before), digest(after), after}
	raw, e := json.Marshal(intent)
	if e != nil {
		return e
	}
	journal := filepath.Join(root, "transaction.json")
	if e = safefile.Write(journal, raw, 0600); e != nil {
		return e
	}
	if e = revisionStep("revision-prepared"); e != nil {
		return e
	}
	if e = safefile.Write(filepath.Join(root, "records.json"), after, 0600); e != nil {
		return e
	}
	if e = revisionStep("revision-records"); e != nil {
		return e
	}
	head, _ := json.Marshal(revisionAnchor{2, r.Authority, r.Server, digest(after)})
	if e = safefile.Write(anchor, head, 0600); e != nil {
		return e
	}
	if e = revisionStep("revision-anchor"); e != nil {
		return e
	}
	if e = os.Remove(journal); e != nil {
		return e
	}
	return safefile.SyncDir(root)
}

// RecoverV2Revision completes only a recorded commit, never a missing authority.
func RecoverV2Revision(c *config.Config) error {
	if e := checkAuthorityBinding(c, "ready"); e != nil {
		return e
	}
	lock, e := revisionLock(c, true)
	if e != nil {
		return e
	}
	defer lock.Close()
	root, anchor := authorityPaths(c)
	data, e := privateRevisionFile(filepath.Join(root, "records.json"))
	if e != nil {
		return e
	}
	headRaw, e := privateRevisionFile(anchor)
	if e != nil {
		return e
	}
	txRaw, e := privateRevisionFile(filepath.Join(root, "transaction.json"))
	if e != nil {
		return e
	}
	var tx revisionIntent
	var head revisionAnchor
	if json.Unmarshal(txRaw, &tx) != nil || json.Unmarshal(headRaw, &head) != nil || tx.Version != 2 || tx.Authority != c.V2Authority.ID || tx.Server != c.ServerID || tx.AfterHash != digest(tx.After) || head.Version != 2 || head.Authority != tx.Authority || head.Server != tx.Server {
		return errors.New("invalid revision recovery intent")
	}
	if _, e = decodeRegistry(c, tx.After); e != nil {
		return e
	}
	if (digest(data) != tx.Before && digest(data) != tx.AfterHash) || (head.Hash != tx.Before && head.Hash != tx.AfterHash) {
		return errors.New("revision recovery conflicts with high-water")
	}
	if e = safefile.Write(filepath.Join(root, "records.json"), tx.After, 0600); e != nil {
		return e
	}
	head.Hash = tx.AfterHash
	headRaw, _ = json.Marshal(head)
	if e = safefile.Write(anchor, headRaw, 0600); e != nil {
		return e
	}
	if e = os.Remove(filepath.Join(root, "transaction.json")); e != nil {
		return e
	}
	return safefile.SyncDir(root)
}

type revisionCheckpoint struct {
	Version                         int
	Authority, Server, RegistryHash string
}

func CheckpointV2Authority(c *config.Config, dest string) (string, error) {
	lock, e := revisionLock(c, false)
	if e != nil {
		return "", e
	}
	defer lock.Close()
	_, raw, e := readRevision(c)
	if e != nil {
		return "", e
	}
	if e = os.Mkdir(dest, 0700); e != nil {
		return "", e
	}
	m := revisionCheckpoint{2, c.V2Authority.ID, c.ServerID, digest(raw)}
	manifest, _ := json.Marshal(m)
	if e = safefile.Write(filepath.Join(dest, "records.json"), raw, 0600); e != nil {
		return "", e
	}
	if e = safefile.Write(filepath.Join(dest, "manifest.json"), manifest, 0600); e != nil {
		return "", e
	}
	if e = safefile.SyncDir(filepath.Dir(dest)); e != nil {
		return "", e
	}
	return digest(manifest), nil
}

// expected must come from an independently retained CURRENT checkpoint, not its DB.
func RepairV2Authority(c *config.Config, snapshot, expected string) error {
	if e := checkAuthorityBinding(c, "ready"); e != nil {
		return e
	}
	manifest, e := privateRevisionFile(filepath.Join(snapshot, "manifest.json"))
	if e != nil {
		return e
	}
	if len(expected) != 64 || digest(manifest) != expected {
		return errors.New("external checkpoint authority mismatch")
	}
	var m revisionCheckpoint
	if json.Unmarshal(manifest, &m) != nil || m.Version != 2 || m.Authority != c.V2Authority.ID || m.Server != c.ServerID {
		return errors.New("checkpoint binding mismatch")
	}
	data, e := privateRevisionFile(filepath.Join(snapshot, "records.json"))
	if e != nil {
		return e
	}
	if digest(data) != m.RegistryHash {
		return errors.New("checkpoint registry mismatch")
	}
	r, e := decodeRegistry(c, data)
	if e != nil {
		return e
	}
	rootForRepair, _ := authorityPaths(c)
	if e = os.Mkdir(rootForRepair, 0700); e != nil && !os.IsExist(e) {
		return e
	}
	repairedLock, e := os.OpenFile(filepath.Join(rootForRepair, "write.lock"), os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	repairedLock.Close()
	lock, e := revisionLock(c, true)
	if e != nil {
		return e
	}
	defer lock.Close()
	root, anchor := authorityPaths(c)
	// A surviving later registry is never overwritten by an older checkpoint.
	if old, e := privateRevisionFile(filepath.Join(root, "records.json")); e == nil {
		previous, e := decodeRegistry(c, old)
		if e != nil {
			return e
		}
		for k, v := range previous.Records {
			next, ok := r.Records[k]
			if !ok || next.Revision < v.Revision || (v.Confirmed != nil && (next.Confirmed == nil || *next.Confirmed < *v.Confirmed)) {
				return errors.New("checkpoint regresses surviving high-water")
			}
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = safefile.Write(filepath.Join(root, "records.json"), data, 0600); e != nil {
		return e
	}
	head, _ := json.Marshal(revisionAnchor{2, m.Authority, m.Server, m.RegistryHash})
	if e = safefile.Write(anchor, head, 0600); e != nil {
		return e
	}
	if e = safefile.Write(c.Path()+".status-v2.anchor", []byte("private-v2-authority:"+m.Authority), 0600); e != nil {
		return e
	}
	return nil
}

type revisionAdoption struct {
	Version                                                 int
	Server, Authority, ConfigHash, RegistryHash, AnchorHash string
}

func AdoptLegacyV2Authority(c *config.Config, envelope []byte, pinned string) error {
	var m revisionAdoption
	if e := recoveryauth.Verify(envelope, pinned, &m); e != nil {
		return e
	}
	return withPublicationLock(func() error {
		if e := c.RefreshIfChanged(); e != nil {
			return e
		}
		if m.Version != 1 || m.Server != c.ServerID || !validPublicationID(m.Authority) {
			return errors.New("invalid adoption identity")
		}
		root, anchor := authorityPaths(c)
		archive := c.Path() + ".status-v2.adoption"
		if c.V2Authority != nil && c.V2Authority.State == "ready" {
			recorded, e := os.ReadFile(filepath.Join(archive, "signed-manifest.json"))
			if e != nil || digest(recorded) != digest(envelope) || c.V2Authority.ID != m.Authority {
				return errors.New("another authority already exists")
			}
			return ValidateV2Authority(c)
		}
		fresh := c.V2Authority == nil || c.V2Authority.State == "legacy"
		if fresh {
			raw, e := os.ReadFile(c.Path())
			if e != nil {
				return e
			}
			if digest(raw) != m.ConfigHash {
				return errors.New("adoption source config changed")
			}
			old, e := privateRevisionFile(filepath.Join(root, "records.json"))
			if e != nil {
				return e
			}
			oldAnchor, e := privateRevisionFile(c.Path() + ".status-v2.anchor")
			if e != nil {
				return e
			}
			if digest(old) != m.RegistryHash || digest(oldAnchor) != m.AnchorHash || string(oldAnchor) != "private-v2-revisions-v1" {
				return errors.New("legacy source differs from authenticated reference")
			}
			if e = os.Mkdir(archive, 0700); e != nil && !os.IsExist(e) {
				return e
			}
			for _, item := range []struct {
				name  string
				value []byte
			}{{"signed-manifest.json", envelope}, {"records.json", old}, {"anchor", oldAnchor}} {
				path := filepath.Join(archive, item.name)
				if b, e := os.ReadFile(path); e == nil {
					if digest(b) != digest(item.value) {
						return errors.New("another adoption archive exists")
					}
				} else if os.IsNotExist(e) {
					if e = safefile.Write(path, item.value, 0600); e != nil {
						return e
					}
				} else {
					return e
				}
			}
			c.V2Authority = &config.RevisionAuthority{ID: m.Authority, Server: m.Server, State: "adopting"}
			if e = c.Save(); e != nil {
				return e
			}
			if e = revisionStep("adopt-binding"); e != nil {
				return e
			}
		} else if c.V2Authority.State != "adopting" || c.V2Authority.ID != m.Authority || c.V2Authority.Server != m.Server {
			return errors.New("another revision transition exists")
		}
		signed, e := privateRevisionFile(filepath.Join(archive, "signed-manifest.json"))
		if e != nil {
			return e
		}
		if digest(signed) != digest(envelope) {
			return errors.New("adoption request changed")
		}
		old, e := privateRevisionFile(filepath.Join(archive, "records.json"))
		if e != nil {
			return e
		}
		if digest(old) != m.RegistryHash {
			return errors.New("adoption archive changed")
		}
		records := map[string]statusRecord{}
		if json.Unmarshal(old, &records) != nil {
			return errors.New("legacy registry invalid")
		}
		r := revisionRegistry{2, m.Authority, m.Server, records}
		data, _ := json.Marshal(r)
		if _, e = decodeRegistry(c, data); e != nil {
			return e
		}
		if e = safefile.Write(filepath.Join(root, "records.json"), data, 0600); e != nil {
			return e
		}
		head, _ := json.Marshal(revisionAnchor{2, m.Authority, m.Server, digest(data)})
		if e = safefile.Write(anchor, head, 0600); e != nil {
			return e
		}
		if e = safefile.Write(c.Path()+".status-v2.anchor", []byte("private-v2-authority:"+m.Authority), 0600); e != nil {
			return e
		}
		if e = revisionStep("adopt-files"); e != nil {
			return e
		}
		c.V2Authority = &config.RevisionAuthority{ID: m.Authority, Server: m.Server, State: "ready"}
		if e = c.Save(); e != nil {
			return fmt.Errorf("adoption stays held: %w", e)
		}
		return revisionStep("adopt-ready")
	})
}
