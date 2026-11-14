package coreinstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreplan"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

var supplementalHashes = map[string]map[string]string{
	"sing-box": {"amd64": "b9986e21a1b70a3648f0ff224769e0bf9b7659e700dc27b24df87179a2562626", "arm64": "a52d4ca505d754b2c6c6aabe4361664649252cddaf79a7a25c9dd546e861640a"},
	"mita":     {"amd64": "d59c7758446fc1c9a73323e46261b783e2856c8951ca699d97b712e34f192154", "arm64": "c112ba9df78da4f3bb2e705e62aa4efb1daa8e9c49f90c732c18e01d98cf3645"},
}

func supplementalPath(kind string) string {
	return filepath.Join(config.DefaultBinDir, "rovunet-"+kind)
}
func verifySupplemental(kind, path string) error {
	expected := supplementalHashes[kind][runtime.GOARCH]
	if expected == "" {
		return fmt.Errorf("unsupported supplemental runtime architecture")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != expected {
		return fmt.Errorf("supplemental runtime digest mismatch")
	}
	return nil
}
func ensureSupplemental(kind string) error {
	path := supplementalPath(kind)
	if _, e := os.Stat(path); e == nil {
		return verifySupplemental(kind, path)
	} else if !os.IsNotExist(e) {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	temp, e := binary.Download(fmt.Sprintf("%s/%s-linux-%s", binary.RuntimeDownloadBase(), kind, runtime.GOARCH), filepath.Dir(path))
	if e != nil {
		return e
	}
	defer os.Remove(temp)
	if e = verifySupplemental(kind, temp); e != nil {
		return e
	}
	if e = os.Chmod(temp, 0755); e != nil {
		return e
	}
	return os.Rename(temp, path)
}
func supplementalUnit(name, dir, protocol, condition, confirm string) *service.Unit {
	kind := coreplan.SupplementalRuntime(protocol)
	u := &service.Unit{Name: ServiceName(name), Description: "Rovunet Panel " + protocol + ": " + name, ExecCondition: condition, ExecStartPost: confirm, User: "root", Group: config.SystemGroup, After: "network-online.target", Restart: "on-failure", WorkingDir: dir}
	if kind == "sing-box" {
		u.ExecStart = supplementalPath(kind) + " run -c " + filepath.Join(dir, "server.json")
	} else {
		u.UMask = "0077"
		u.StateDirectory = ServiceName(name)
		u.BindPaths = []string{"/var/lib/" + u.StateDirectory + ":/var/lib/mita"}
		u.ExecStart = supplementalPath(kind) + " run"
		u.Environment = []string{"MITA_CONFIG_JSON_FILE=" + filepath.Join(dir, "server.json"), "MITA_UDS_PATH=" + filepath.Join(dir, "mita.sock"), "MITA_LOG_NO_TIMESTAMP=1", "MITA_INSECURE_UDS=1"}
	}
	return u
}
func testSupplemental(path, protocol string) error {
	kind := coreplan.SupplementalRuntime(protocol)
	if e := verifySupplemental(kind, supplementalPath(kind)); e != nil {
		return e
	}
	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if kind == "sing-box" {
		cmd = exec.CommandContext(ctx, supplementalPath(kind), "check", "-c", path)
	} else {
		cmd = exec.CommandContext(ctx, "unshare", "--mount", "--net", "--pid", "--fork", "--kill-child", "sh", "-c", mitaValidationScript, "mita-validation", supplementalPath(kind), path)
	}
	if _, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("supplemental runtime rejected private configuration: %w", e)
	}
	return nil
}

// Upstream mita has no offline check command: describe config queries the live
// daemon. Validate a real startup in private network/PID/mount namespaces, with
// ephemeral metrics and RPC files, never touching an installed instance.
const mitaValidationScript = `set -eu
mount --make-rprivate /
mount -t tmpfs tmpfs /var/lib
mount -t tmpfs tmpfs /run
umask 077
export MITA_CONFIG_JSON_FILE="$2" MITA_UDS_PATH=/run/rovunet-mita-check.sock MITA_LOG_NO_TIMESTAMP=1 MITA_INSECURE_UDS=1
"$1" run >/dev/null 2>&1 &
child=$!
trap 'kill "$child" 2>/dev/null || true; wait "$child" 2>/dev/null || true' EXIT
n=0
while [ "$n" -lt 50 ]; do
  kill -0 "$child" 2>/dev/null || exit 1
  if "$1" status 2>&1 | grep -q '"RUNNING"'; then exit 0; fi
  n=$((n+1))
  sleep 0.1
done
exit 1
`
