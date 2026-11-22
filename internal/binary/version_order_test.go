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

func TestVersion210UpdatesPublic202AndPrivate203(t *testing.T) {
	for _, current := range []string{"2.0.2", "2.0.3"} {
		if err := CheckUpgradeOrder(current, "v2.1.0"); err != nil {
			t.Fatalf("%s cannot update to the replacement release: %v", current, err)
		}
	}
	if CheckUpgradeOrder("2.0.3", "v2.0.2") == nil {
		t.Fatal("the previous public version must still be rejected as a downgrade")
	}
	if CheckUpgradeOrder("2.1.0", "v2.0.3") == nil {
		t.Fatal("the new release must not silently revert to the private version")
	}
}
