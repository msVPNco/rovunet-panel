package binary

import "testing"

func TestAutomaticUpdateCannotReplayAnOlderRelease(t *testing.T) {
	for _, v := range []string{"v1.20.9", "v1.9.99", "v0.99.99", "v1.021.2", "not-a-version"} {
		if rejectOlderRelease("1.21.2", v) == nil {
			t.Fatal("unsafe release order accepted")
		}
	}
	for _, v := range []string{"v1.21.2", "v1.21.3", "v1.22.0", "v2.0.0"} {
		if err := rejectOlderRelease("1.21.2", v); err != nil {
			t.Fatal(err)
		}
	}
}
