package system

import (
	"strings"
	"testing"
)

func TestSSHUpgradeKeepsAdministratorAndCustomDestinationPolicy(t *testing.T) {
	admin := "Match User administrator\n    AuthenticationMethods publickey\n    AllowTcpForwarding no\n"
	old := "Port 22\n# Rovunet SSH tunnel users\nMatch Group rovunet-ssh\n    AllowTcpForwarding yes\n    PermitOpen example.test:443\n" + admin
	next := withRovunetSSHConfig(old)
	if !strings.Contains(next, admin) || !strings.Contains(next, "PermitOpen example.test:443") {
		t.Fatal("administrator or custom SSH restriction was removed")
	}
	if !strings.Contains(next, "AllowTcpForwarding local") || !strings.Contains(next, "PermitTTY no") {
		t.Fatal("unneeded SSH forwarding/session access retained")
	}
	if strings.Count(next, "# Rovunet SSH tunnel users") != 1 || withRovunetSSHConfig(next) != next {
		t.Fatal("SSH upgrade is not idempotent")
	}
}
