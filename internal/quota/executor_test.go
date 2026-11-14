package quota

import (
	"os"
	"os/exec"
	"runtime"
)

// A user-mode emulator does not automatically register a kernel ELF handler.
// Let ARM acceptance tests explicitly launch their crash-helper children via
// that same emulator. Native test runs continue to execute themselves directly.
func quotaTestCommand(args ...string) *exec.Cmd {
	if runtime.GOARCH == "arm64" && os.Getenv("ROVUNET_TEST_ARM_EXECUTOR") != "" {
		return exec.Command(os.Getenv("ROVUNET_TEST_ARM_EXECUTOR"), append([]string{os.Args[0]}, args...)...)
	}
	return exec.Command(os.Args[0], args...)
}
