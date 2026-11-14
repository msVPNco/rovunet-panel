package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/system"
	"github.com/msVPNco/rovunet-panel/internal/transport"
)

const V2Unit = "rovunet-subscription-v2.service"

func ServeSubscriptionV2(ctx context.Context, path string) error {
	cfg, e := config.LoadFrom(path)
	if e != nil {
		return e
	}
	if !cfg.SubscriptionV2 {
		return errors.New("subscription v2 is disabled")
	}
	if e = ValidateV2Authority(cfg); e != nil {
		return errors.New("revision authority was not prepared")
	}
	socket := config.SubscriptionV2Socket
	if info, e := os.Lstat(socket); e == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return errors.New("renderer socket path is not a socket")
		}
		c, err := net.DialTimeout("unix", socket, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return errors.New("renderer already listening")
		}
		if !errors.Is(err, syscall.ECONNREFUSED) {
			return err
		}
		if e = os.Remove(socket); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	ln, e := net.Listen("unix", socket)
	if e != nil {
		return e
	}
	defer ln.Close()
	group, e := user.LookupGroup(config.SystemGroup)
	if e != nil {
		return e
	}
	gid, e := strconv.Atoi(group.Gid)
	if e != nil {
		return e
	}
	if e = os.Chown(socket, 0, gid); e != nil {
		return e
	}
	if e = os.Chmod(socket, 0660); e != nil {
		return e
	}
	srv := &http.Server{Handler: SubscriptionMetadataHandler(ResolveInstalledV2(path)), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		end, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e := srv.Shutdown(end); e != nil {
			srv.Close()
		}
	}()
	e = srv.Serve(ln)
	if !errors.Is(e, http.ErrServerClosed) {
		return e
	}
	<-shutdownDone
	return nil
}
func PrepareV2Authority(cfg *config.Config) error { return ValidateV2Authority(cfg) }
func V2UnitContent(cfg *config.Config) (string, error) {
	if cfg.Path() != config.DefaultConfigFile {
		return "", errors.New("deployment requires canonical installed configuration")
	}
	writable := []string{cfg.Path() + ".status-v2", cfg.Path() + ".status-v2.anchor"}
	seen := map[string]bool{}
	all := append([]config.UserConfig(nil), cfg.Users...)
	for _, a := range cfg.Archived {
		all = append(all, a.UserConfig)
	}
	for _, u := range all {
		if u.SQLiteMeter == nil {
			continue
		}
		dir := u.SQLiteMeter.Directory
		if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || len(dir) > 200 || containsUnitControl(dir) {
			return "", errors.New("unsupported DB deployment path")
		}
		if !seen[dir] {
			writable = append(writable, dir)
			seen[dir] = true
		}
	}
	content := `[Unit]
Description=Rovunet private subscription v2 renderer
After=local-fs.target
[Service]
Type=simple
User=root
Group=root
SupplementaryGroups=rovunet
ExecStart=/usr/local/bin/rovunet subscription-v2 serve
Restart=on-failure
RestartSec=1
UMask=0077
RuntimeDirectory=rovunet-subscription-v2
RuntimeDirectoryMode=0711
NoNewPrivileges=yes
CapabilityBoundingSet=
PrivateNetwork=yes
PrivateDevices=yes
ProtectSystem=strict
ProtectHome=yes
RestrictAddressFamilies=AF_UNIX
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
ReadWritePaths=/run/rovunet-subscription-v2
`
	for _, p := range writable {
		content += "ReadWritePaths=" + p + "\n"
	}
	return content + "[Install]\nWantedBy=multi-user.target\n", nil
}
func containsUnitControl(s string) bool {
	for _, r := range s {
		if r == '%' || r == '"' || r == '\\' || r <= 32 {
			return true
		}
	}
	return false
}
func ApplyV2Deployment(cfg *config.Config) error {
	if !cfg.SubscriptionV2 {
		if cfg.V2Authority != nil {
			return ValidateV2Authority(cfg)
		}
		return nil
	}
	if e := PrepareV2Authority(cfg); e != nil {
		return e
	}
	if e := system.EnsureUser(); e != nil {
		return e
	}
	body, e := V2UnitContent(cfg)
	if e != nil {
		return e
	}
	if e = safefile.Write("/etc/systemd/system/"+V2Unit, []byte(body), 0644); e != nil {
		return e
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", "--now", V2Unit}, {"restart", V2Unit}} {
		if b, e := exec.Command("systemctl", args...).CombinedOutput(); e != nil {
			return fmt.Errorf("renderer lifecycle failed: %w (%d bytes diagnostic)", e, len(b))
		}
	}
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if t.Enabled && t.Transport == config.TransportNaive {
			if e = transport.RegenerateService(t, cfg); e != nil {
				return e
			}
		}
	}
	return nil
}
func SetV2Deployment(cfg *config.Config, enabled bool) error {
	if enabled {
		if e := ValidateV2Authority(cfg); e != nil {
			return e
		}
	}
	if !enabled && cfg.SubscriptionV2 && cfg.V2Authority == nil {
		cfg.V2Authority = &config.RevisionAuthority{Server: cfg.ServerID, State: "legacy"}
	}
	cfg.SubscriptionV2 = enabled
	if e := cfg.Save(); e != nil {
		return e
	}
	if enabled {
		return ApplyV2Deployment(cfg)
	}
	// Remove the public route before stopping the renderer. Never point to a
	// stale public file if the dynamic authority is absent.
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		if t.Enabled && t.Transport == config.TransportNaive {
			if e := transport.RegenerateService(t, cfg); e != nil {
				return e
			}
		}
	}
	if b, e := exec.Command("systemctl", "disable", "--now", V2Unit).CombinedOutput(); e != nil {
		return fmt.Errorf("renderer stop failed: %w (%d bytes diagnostic)", e, len(b))
	}
	return nil
}
func RendererSignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
