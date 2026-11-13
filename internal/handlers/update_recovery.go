package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/backup"
	"github.com/msVPNco/rovunet-panel/internal/binary"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/safefile"
	"github.com/msVPNco/rovunet-panel/internal/service"
)

func prepareUpdateRecovery(cfg *config.Config) (string, error) {
	text, skipped, err := encodeBackup(cfg)
	if err != nil {
		return "", err
	}
	if len(skipped) > 0 {
		return "", errors.New("update backup omitted files; inspect them before updating")
	}
	if _, err = backup.Decode(text); err != nil {
		return "", fmt.Errorf("update backup validation failed: %w", err)
	}
	root := filepath.Join(config.DefaultConfigDir, "update-backups")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(root, time.Now().UTC().Format("20060102T150405Z-"))
	if err != nil {
		return "", err
	}
	if err := safefile.Write(filepath.Join(dir, "panel.backup"), []byte(text), 0600); err != nil {
		return "", err
	}
	if err := safefile.Copy(cfg.Path(), filepath.Join(dir, "config.original.json"), 0600); err != nil {
		return "", err
	}
	units := filepath.Join(dir, "units")
	if err := os.Mkdir(units, 0700); err != nil {
		return "", err
	}
	entries, err := os.ReadDir("/etc/systemd/system")
	if err != nil {
		return "", err
	}
	states := map[string]string{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasPrefix(e.Name(), "rovunet-") || (!strings.HasSuffix(e.Name(), ".service") && !strings.HasSuffix(e.Name(), ".timer")) {
			continue
		}
		if err := safefile.Copy(filepath.Join("/etc/systemd/system", e.Name()), filepath.Join(units, e.Name()), 0600); err != nil {
			return "", err
		}
		b, err := exec.Command("systemctl", "show", e.Name(), "--property=ActiveState,UnitFileState").Output()
		if err != nil {
			return "", fmt.Errorf("cannot snapshot state of owned unit %s", e.Name())
		}
		states[e.Name()] = strings.TrimSpace(string(b))
	}
	b, _ := json.MarshalIndent(states, "", "  ")
	if err := safefile.Write(filepath.Join(dir, "unit-states.json"), b, 0600); err != nil {
		return "", err
	}
	captures := map[string]string{}
	for _, path := range []string{"/etc/ssh/sshd_config"} {
		if _, err := os.Stat(path); err == nil {
			if err := safefile.Copy(path, filepath.Join(dir, "sshd_config.before"), 0600); err != nil {
				return "", err
			}
			captures[path] = "captured"
		} else if os.IsNotExist(err) {
			captures[path] = "absent"
		} else {
			return "", err
		}
	}
	for _, tool := range []string{"iptables-save", "ip6tables-save"} {
		if _, err := exec.LookPath(tool); err != nil {
			captures[tool] = "unavailable; not captured"
			continue
		}
		b, err := exec.Command(tool).Output()
		if err != nil {
			return "", fmt.Errorf("cannot snapshot %s", tool)
		}
		if err := safefile.Write(filepath.Join(dir, tool+".before"), b, 0600); err != nil {
			return "", err
		}
		captures[tool] = "captured"
	}
	b, _ = json.MarshalIndent(captures, "", "  ")
	if err := safefile.Write(filepath.Join(dir, "capture-status.json"), b, 0600); err != nil {
		return "", err
	}
	return dir, nil
}

func runStrictPostUpdate(path string, tx *binary.UpdateTransaction) error {
	if err := tx.SavePhase("post-update-pending"); err != nil {
		return err
	}
	c := exec.Command(path, "post-update", "--strict")
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := c.Run(); err != nil {
		return fmt.Errorf("new panel is installed but setup is pending; run rovunet update to resume; recovery snapshot: %s (previous binary retained, do not replay old migration code)", tx.Recovery)
	}
	return tx.SavePhase("complete")
}

// CompleteBootstrap clears only a matching installer receipt after setup has
// actually completed. A failure before Rename does not require a reinstall.
func CompleteBootstrap() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	state := filepath.Join(filepath.Dir(path), ".rovunet-install-state.json")
	b, err := os.ReadFile(state)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var receipt struct {
		Schema int    `json:"schema"`
		SHA256 string `json:"sha256"`
	}
	if json.Unmarshal(b, &receipt) != nil || receipt.Schema != 1 {
		return errors.New("invalid bootstrap recovery receipt")
	}
	current, err := binary.FileSHA256(path)
	if err != nil {
		return err
	}
	if current != receipt.SHA256 {
		return errors.New("bootstrap receipt belongs to a different executable; retain it for inspection")
	}
	if err := os.Remove(state); err != nil {
		return err
	}
	return safefile.SyncDir(filepath.Dir(path))
}

func activeService(name string) bool { state, _ := service.Status(name); return state == "active" }
