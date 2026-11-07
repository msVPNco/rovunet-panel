package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

// cloudSSHDConfig is the start of a stock Ubuntu cloud image's sshd_config,
// whose include turns password logins off.
const cloudSSHDConfig = `Include /etc/ssh/sshd_config.d/*.conf

KbdInteractiveAuthentication no
UsePAM yes
`

// oldRovunetBlock is the Match block before password logins were enabled.
const oldRovunetBlock = `
# Rovunet SSH tunnel users
Match Group rovunet-ssh
    AllowTcpForwarding yes
    X11Forwarding no
    AllowAgentForwarding no
    ForceCommand /bin/false
    ClientAliveInterval 30
    ClientAliveCountMax 10
`

func TestRovunetSSHConfigFresh(t *testing.T) {
	got := withRovunetSSHConfig(cloudSSHDConfig)
	if !strings.HasPrefix(got, cloudSSHDConfig) || !strings.HasSuffix(got, sshMatchBlock) {
		t.Fatalf("got:\n%s", got)
	}
	if g, m := strings.Index(got, "# Rovunet global SSH settings"), strings.Index(got, "Match Group"); g < 0 || g > m {
		t.Errorf("global settings missing or after the Match block:\n%s", got)
	}
}

func TestRovunetSSHConfigUpgradesOldBlock(t *testing.T) {
	got := withRovunetSSHConfig(cloudSSHDConfig + sshGlobalSettings + oldRovunetBlock)
	if n := strings.Count(got, "Match Group rovunet-ssh"); n != 1 {
		t.Fatalf("%d Match blocks:\n%s", n, got)
	}
	if !strings.HasSuffix(got, sshMatchBlock) || strings.Count(got, "# Rovunet global SSH settings") != 1 {
		t.Errorf("got:\n%s", got)
	}
}

func TestRovunetSSHConfigIdempotent(t *testing.T) {
	once := withRovunetSSHConfig(cloudSSHDConfig)
	if twice := withRovunetSSHConfig(once); twice != once {
		t.Errorf("second pass changed the config:\n%s", twice)
	}
}

func TestRovunetSSHConfigKeepsForeignBlock(t *testing.T) {
	in := cloudSSHDConfig + sshGlobalSettings + "\nMatch Group rovunet-ssh\n    AllowTcpForwarding yes\n"
	if got := withRovunetSSHConfig(in); got != in {
		t.Errorf("hand-written block replaced:\n%s", got)
	}
}

func TestRovunetSSHConfigGlobalsBeforeOtherMatch(t *testing.T) {
	in := cloudSSHDConfig + "\nMatch User backup\n    ForceCommand /usr/bin/rsync\n"
	got := withRovunetSSHConfig(in)
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

func TestListGroupUsersIncludesPrimaryGroupMembers(t *testing.T) {
	dir := t.TempDir()
	group := dir + "/group"
	passwd := dir + "/passwd"
	_ = os.WriteFile(group, []byte("root:x:0:\nrovunet-ssh:x:998:extra\n"), 0600)
	_ = os.WriteFile(passwd, []byte("root:x:0:0::/root:/bin/bash\nalice:x:997:998::/:/bin/false\nbob:x:996:100::/:/bin/sh\n"), 0600)
	got, err := listGroupUsers(group, passwd, "rovunet-ssh")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "extra" || got[1] != "alice" {
		t.Fatalf("got %v, want [extra alice]", got)
	}
}

// A pre-rename install's sshd_config blocks must be taken over, not left
// behind beside a second set of blocks.
func TestSSHConfigTakesOverLegacyBlocks(t *testing.T) {
	old := "Port 22\n" + strings.Replace(strings.Replace(strings.Replace(sshGlobalSettings+sshMatchBlock,
		"# Rovunet global SSH settings", "# Slip"+"Gate global SSH settings", 1),
		"# Rovunet SSH tunnel users", "# Slip"+"Gate SSH tunnel users", 1),
		"Match Group "+config.SSHGroup, "Match Group slip"+"gate-ssh", 1)
	got := withRovunetSSHConfig(old)
	if strings.Contains(got, "Slip"+"Gate") || strings.Contains(got, "slip"+"gate") {
		t.Fatalf("legacy names left:\n%s", got)
	}
	if n := strings.Count(got, "Match Group "+config.SSHGroup); n != 1 {
		t.Fatalf("%d Match blocks, want 1:\n%s", n, got)
	}
	if n := strings.Count(got, "# Rovunet global SSH settings"); n != 1 {
		t.Fatalf("%d global blocks, want 1:\n%s", n, got)
	}
}
