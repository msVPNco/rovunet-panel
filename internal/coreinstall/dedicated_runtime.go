package coreinstall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
)

func dedicatedXrayPath() string { return filepath.Join(filepath.Dir(XrayPath), "rovunet-xray-managed") }

// XrayForCore preserves the runtime choice in options, including after restore.
func XrayForCore(name string) string {
	var o coreplan.Options
	if readJSON(filepath.Join(Root, name, optionsFile), &o) == nil && o.DedicatedXray {
		return dedicatedXrayPath()
	}
	return XrayPath
}

// The pinned client understands both the old TCP and new Unix-socket APIs.
func apiXrayPath() string {
	if _, err := os.Stat(dedicatedXrayPath()); err == nil {
		return dedicatedXrayPath()
	}
	return XrayPath
}

func ensureOptionsEngine(o coreplan.Options) error {
	if !o.DedicatedXray {
		return ensureEngine(o.Protocol)
	}
	if !liveUserProtocol(o.Protocol) && !coreplan.BridgeProtocol(o.Protocol) {
		return fmt.Errorf("dedicated runtime is only used for managed Xray protocols")
	}
	path := dedicatedXrayPath()
	if _, err := os.Stat(path); err == nil {
		return verifyXrayFile(path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var temp string
	if verifyGoSource(XrayPath, XrayCommit) == nil {
		// Reuse a clean pinned build, without modifying the legacy file.
		in, err := os.Open(XrayPath)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.CreateTemp(filepath.Dir(path), ".managed-xray-*")
		if err != nil {
			return err
		}
		temp = out.Name()
		defer os.Remove(temp)
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	} else {
		var err error
		temp, err = binary.Download(fmt.Sprintf("%s/xray-%s-%s", binary.RuntimeDownloadBase(), runtime.GOOS, runtime.GOARCH), filepath.Dir(path))
		if err != nil {
			return err
		}
		defer os.Remove(temp)
	}
	if err := os.Chmod(temp, 0755); err != nil {
		return err
	}
	if err := verifyXrayFile(temp); err != nil {
		return err
	}
	defer os.Remove(temp + ".provenance.json")
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	return verifyXrayFile(path)
}

func ensureCoreEngine(name string, protocol string) error {
	var o coreplan.Options
	if err := readJSON(filepath.Join(Root, name, optionsFile), &o); err == nil {
		return ensureOptionsEngine(o)
	}
	return ensureEngine(protocol)
}
