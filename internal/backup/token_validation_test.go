package backup

import (
	"github.com/msVPNco/rovunet-panel/internal/config"
	"testing"
)

func TestRestoreRejectsSubscriptionFilenameTraversal(t *testing.T) {
	for _, token := range []string{"../outside", "/absolute", "token.json", "..\\outside"} {
		b := &Bundle{Format: 1, Config: config.Default()}
		b.Config.Users = []config.UserConfig{{Username: "alice", Password: "pw-valid", SubToken: token}}
		text, err := b.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got, err := Decode(text); err == nil || got != nil {
			t.Fatal("backup with unsafe subscription filename accepted")
		}
	}
}
