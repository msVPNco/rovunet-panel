package system

import (
	"strings"
	"testing"
)

func TestUninstallSSHBlocksPreservesAdministratorRules(t *testing.T) {
	admin := "\nMatch User backup\n    ForceCommand /usr/bin/rsync\n"
	for _, legacy := range []bool{false, true} {
		owned := sshGlobalSettings + sshMatchBlock
		if legacy {
			owned = strings.ReplaceAll(strings.ReplaceAll(owned, "Rovunet", "SlipGate"), "rovunet-ssh", "slipgate-ssh")
		}
		input := cloudSSHDConfig + owned + admin
		clean, err := withoutOwnedSSHConfig(input)
		if err != nil || clean != cloudSSHDConfig+admin {
			t.Fatalf("administrator config changed: %v", err)
		}
		if again, err := withoutOwnedSSHConfig(clean); err != nil || again != clean {
			t.Fatal("cleanup is not idempotent")
		}
	}
	for _, input := range []string{cloudSSHDConfig + sshMatchBlock + "    PermitOpen db:5432\n", cloudSSHDConfig + oldRovunetBlock} {
		if _, err := withoutOwnedSSHConfig(input); err == nil {
			t.Fatal("removed a customized or older block without review")
		}
	}
}
