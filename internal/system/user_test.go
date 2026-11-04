package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cloudSSHDConfig is the start of a stock Ubuntu cloud image's sshd_config,
// whose include turns password logins off.
const cloudSSHDConfig = `Include /etc/ssh/sshd_config.d/*.conf

KbdInteractiveAuthentication no
UsePAM yes
`

// oldSlipgateBlock is the Match block before password logins were enabled.
const oldSlipgateBlock = `
# Rovunet SSH tunnel users
Match Group rovunet-ssh
    AllowTcpForwarding yes
    X11Forwarding no
    AllowAgentForwarding no
    ForceCommand /bin/false
    ClientAliveInterval 30
    ClientAliveCountMax 10
`

func TestSlipgateSSHConfigFresh(t *testing.T) {
	got := withPanelSSHConfig(cloudSSHDConfig)
	if !strings.HasPrefix(got, cloudSSHDConfig) || !strings.HasSuffix(got, sshMatchBlock) {
		t.Fatalf("got:\n%s", got)
	}
	if g, m := strings.Index(got, "# Rovunet global SSH settings"), strings.Index(got, "Match Group"); g < 0 || g > m {
		t.Errorf("global settings missing or after the Match block:\n%s", got)
	}
}

func TestSlipgateSSHConfigUpgradesOldBlock(t *testing.T) {
	got := withPanelSSHConfig(cloudSSHDConfig + sshGlobalSettings + oldSlipgateBlock)
	if n := strings.Count(got, "Match Group rovunet-ssh"); n != 1 {
		t.Fatalf("%d Match blocks:\n%s", n, got)
	}
	if !strings.HasSuffix(got, sshMatchBlock) || strings.Count(got, "# Rovunet global SSH settings") != 1 {
		t.Errorf("got:\n%s", got)
	}
}

func TestSlipgateSSHConfigIdempotent(t *testing.T) {
	once := withPanelSSHConfig(cloudSSHDConfig)
	if twice := withPanelSSHConfig(once); twice != once {
		t.Errorf("second pass changed the config:\n%s", twice)
	}
}

func TestSlipgateSSHConfigKeepsForeignBlock(t *testing.T) {
	in := cloudSSHDConfig + sshGlobalSettings + "\nMatch Group rovunet-ssh\n    AllowTcpForwarding yes\n"
	if got := withPanelSSHConfig(in); got != in {
		t.Errorf("hand-written block replaced:\n%s", got)
	}
}

func TestSlipgateSSHConfigGlobalsBeforeOtherMatch(t *testing.T) {
	in := cloudSSHDConfig + "\nMatch User backup\n    ForceCommand /usr/bin/rsync\n"
	got := withPanelSSHConfig(in)
	if g, m := strings.Index(got, "# Rovunet global SSH settings"), strings.Index(got, "Match User backup"); g < 0 || g > m {
		t.Errorf("global settings after an existing Match block:\n%s", got)
	}
}

// TestEnsureSSHDRunDir: the privilege separation directory sshd -t needs is
// created with sshd's mode when missing, and an existing one is left alone.
func TestEnsureSSHDRunDir(t *testing.T) {
	old := sshdRunDir
	defer func() { sshdRunDir = old }()
	sshdRunDir = filepath.Join(t.TempDir(), "sshd")

	if err := ensureSSHDRunDir(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(sshdRunDir)
	if err != nil || !st.IsDir() || st.Mode().Perm() != 0755 {
		t.Fatalf("got %v %v", st, err)
	}
	if err := ensureSSHDRunDir(); err != nil {
		t.Fatalf("second call: %v", err)
	}
}

// TestLegacySSHBlockMigrates: the block a SlipGate install wrote becomes the
// current block once its markers and group are renamed, without duplicates.
func TestLegacySSHBlockMigrates(t *testing.T) {
	legacy := strings.NewReplacer(
		"# Rovunet global SSH settings", "# SlipGate global SSH settings",
		"# Rovunet SSH tunnel users", "# SlipGate SSH tunnel users",
		"Match Group rovunet-ssh", "Match Group slipgate-ssh",
	).Replace(withPanelSSHConfig(cloudSSHDConfig))
	if !strings.Contains(legacy, "Match Group slipgate-ssh") {
		t.Fatal("test setup: expected a SlipGate block")
	}
	got := withPanelSSHConfig(legacySSHMarkers.Replace(legacy))
	if got != withPanelSSHConfig(cloudSSHDConfig) {
		t.Fatalf("migrated config differs from a fresh one:\n%s", got)
	}
	if strings.Contains(got, "slipgate") || strings.Contains(got, "SlipGate") {
		t.Fatalf("old names left:\n%s", got)
	}
}
