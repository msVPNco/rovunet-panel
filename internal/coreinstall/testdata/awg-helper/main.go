// Test helper; built explicitly by scripts/test-managed-awg.py's caller.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
)

func main() {
	coreinstall.Root = os.Getenv("TEST_CORE_ROOT")
	coreinstall.AWGGoPath = os.Getenv("TEST_AWG_GO")
	coreinstall.AWGToolPath = os.Getenv("TEST_AWG_TOOL")
	if len(os.Args) != 3 {
		log.Fatal("mode and test core name required")
	}
	var err error
	if os.Args[1] == "sync" {
		err = coreinstall.SyncAWG(os.Args[2])
	} else {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
		defer cancel()
		err = coreinstall.RunAWG(ctx, os.Args[2])
	}
	if err != nil {
		log.Fatal(err)
	}
}
