package handlers

import "testing"

func TestOwnerFromSS(t *testing.T) {
	line := `udp UNCONN 0 0 0.0.0.0:8443 0.0.0.0:* users:(("xray",pid=812,fd=7))` + "\n"
	if got := ownerFromSS(line); got != " (xray)" {
		t.Fatalf("got %q", got)
	}
	if got := ownerFromSS(""); got != "" {
		t.Fatalf("empty output gave %q", got)
	}
	if got := ownerFromSS("udp UNCONN 0 0 0.0.0.0:53 0.0.0.0:*\n"); got != "" {
		t.Fatalf("hidden owner gave %q", got)
	}
}
