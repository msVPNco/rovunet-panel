package coreinstall

import (
	"github.com/msVPNco/rovunet-panel/internal/network"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"os"
)

func cleanupCreated(name, dir string, port int, proto string, firewall bool) error {
	if err := service.Stop(ServiceName(name)); err != nil {
		return err
	}
	if err := service.Remove(ServiceName(name)); err != nil {
		return err
	}
	if firewall {
		if err := network.ReleaseOwnedPort("core:"+name, port, proto); err != nil {
			return err
		}
	}
	return os.RemoveAll(dir)
}
