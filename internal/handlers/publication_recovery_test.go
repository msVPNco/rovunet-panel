package handlers

import (
	"errors"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRotationAcrossProcessExit(t *testing.T) {
	useLocalPublicationTLS(t)
	if file := os.Getenv("ROVU_ROTATE_TEST_CFG"); file != "" {
		transport.SubscriptionDir = os.Getenv("ROVU_ROTATE_TEST_ROOT")
		cfg, e := config.LoadFrom(file)
		if e != nil {
			t.Fatal(e)
		}
		publicationBarrier = func(point string) error {
			if point == os.Getenv("ROVU_ROTATE_TEST_PHASE") {
				os.Exit(23)
			}
			return nil
		}
		if e = RotateSubscriptionLink(cfg, "alice", discardOutput{}); e != nil {
			t.Fatal(e)
		}
		return
	}
	for _, phase := range []string{"prepared", "bridge", "selected", "retired"} {
		t.Run(phase, func(t *testing.T) {
			isolatedSubscriptions(t)
			cfg := subTestConfig()
			file := filepath.Join(t.TempDir(), "config.json")
			if e := cfg.SaveTo(file); e != nil {
				t.Fatal(e)
			}
			if e := WriteSubscriptions(cfg, discardOutput{}); e != nil {
				t.Fatal(e)
			}
			old := cfg.Users[0].SubToken
			witness := cfg.Users[1].SubToken
			c := exec.Command(os.Args[0], "-test.run=^TestRotationAcrossProcessExit$")
			c.Env = append(os.Environ(), "ROVU_ROTATE_TEST_CFG="+file, "ROVU_ROTATE_TEST_ROOT="+transport.SubscriptionDir, "ROVU_ROTATE_TEST_PHASE="+phase)
			e := c.Run()
			var exit *exec.ExitError
			if !errors.As(e, &exit) || exit.ExitCode() != 23 {
				t.Fatal(e)
			}
			fresh, e := config.LoadFrom(file)
			if e != nil {
				t.Fatal(e)
			}
			for _, suffix := range aliasSuffixes {
				for _, token := range []string{fresh.Users[0].SubToken, witness} {
					if _, e = os.Stat(filepath.Join(publicDir(), token+suffix)); e != nil {
						t.Fatal(e)
					}
				}
			}
			if e = WriteSubscriptions(cfg, discardOutput{}); e != nil {
				t.Fatal(e)
			}
			if cfg.Users[0].SubToken == old {
				t.Fatal("did not recover selection")
			}
			for _, suffix := range aliasSuffixes {
				if _, e = os.Stat(filepath.Join(publicDir(), old+suffix)); !os.IsNotExist(e) {
					t.Fatal("old capability retained")
				}
			}
		})
	}
}
