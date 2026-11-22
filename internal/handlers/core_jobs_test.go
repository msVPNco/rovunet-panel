package handlers

import "testing"

func TestCoreJobsAddASecondRealityCore(t *testing.T) {
	got := coreJobs([]string{"xhttp", "reality", "hysteria2"})
	want := []coreJob{{"xhttp", "xhttp"}, {"reality", "reality"}, {"reality2", "reality"}, {"hysteria2", "hysteria2"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if n := len(coreJobs([]string{"xhttp", "wireguard"})); n != 2 {
		t.Fatalf("no REALITY, no second core: %d jobs", n)
	}
	if len(coreJobs(nil)) != 0 {
		t.Fatal("nothing chosen, nothing installed")
	}
}

func TestShadowsocksWizardUsesCompleteSetupNames(t *testing.T) {
	got := coreJobs([]string{"ss2022-aes128", "ss-aead-chacha20"})
	want := []coreJob{{"managed-ss2022-aes128", "ss2022-aes128"}, {"managed-ss-aead-chacha20", "ss-aead-chacha20"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
