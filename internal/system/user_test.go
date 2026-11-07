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
# SlipGate SSH tunnel users
Match Group slipgate-ssh
    AllowTcpForwarding yes
    X11Forwarding no
    AllowAgentForwarding no
    ForceCommand /bin/false
    ClientAliveInterval 30
    ClientAliveCountMax 10
`

func TestSlipgateSSHConfigFresh(t *testing.T) {
	got := withSlipgateSSHConfig(cloudSSHDConfig)
	if !strings.HasPrefix(got, cloudSSHDConfig) || !strings.HasSuffix(got, sshMatchBlock) {
		t.Fatalf("got:\n%s", got)
	}
	if g, m := strings.Index(got, "# SlipGate global SSH settings"), strings.Index(got, "Match Group"); g < 0 || g > m {
		t.Errorf("global settings missing or after the Match block:\n%s", got)
	}
}

func TestSlipgateSSHConfigUpgradesOldBlock(t *testing.T) {
	got := withSlipgateSSHConfig(cloudSSHDConfig + sshGlobalSettings + oldSlipgateBlock)
	if n := strings.Count(got, "Match Group slipgate-ssh"); n != 1 {
		t.Fatalf("%d Match blocks:\n%s", n, got)
	}
	if !strings.HasSuffix(got, sshMatchBlock) || strings.Count(got, "# SlipGate global SSH settings") != 1 {
		t.Errorf("got:\n%s", got)
	}
}

func TestSlipgateSSHConfigIdempotent(t *testing.T) {
	once := withSlipgateSSHConfig(cloudSSHDConfig)
	if twice := withSlipgateSSHConfig(once); twice != once {
		t.Errorf("second pass changed the config:\n%s", twice)
	}
}

func TestSlipgateSSHConfigKeepsForeignBlock(t *testing.T) {
	in := cloudSSHDConfig + sshGlobalSettings + "\nMatch Group slipgate-ssh\n    AllowTcpForwarding yes\n"
	if got := withSlipgateSSHConfig(in); got != in {
		t.Errorf("hand-written block replaced:\n%s", got)
	}
}

func TestSlipgateSSHConfigGlobalsBeforeOtherMatch(t *testing.T) {
	in := cloudSSHDConfig + "\nMatch User backup\n    ForceCommand /usr/bin/rsync\n"
	got := withSlipgateSSHConfig(in)
	if g, m := strings.Index(got, "# SlipGate global SSH settings"), strings.Index(got, "Match User backup"); g < 0 || g > m {
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
	_ = os.WriteFile(group, []byte("root:x:0:\nslipgate-ssh:x:998:extra\n"), 0600)
	_ = os.WriteFile(passwd, []byte("root:x:0:0::/root:/bin/bash\nalice:x:997:998::/:/bin/false\nbob:x:996:100::/:/bin/sh\n"), 0600)
	got, err := listGroupUsers(group, passwd, "slipgate-ssh")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "extra" || got[1] != "alice" {
		t.Fatalf("got %v, want [extra alice]", got)
	}
}
