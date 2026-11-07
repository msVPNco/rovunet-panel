package handlers

import "testing"

func TestMovedPath(t *testing.T) {
	old, nw := "/etc/slipgate/tunnels/a", "/etc/slipgate/tunnels/b"
	cases := map[string]string{
		"/etc/slipgate/tunnels/a/cert.pem": "/etc/slipgate/tunnels/b/cert.pem",
		"/etc/letsencrypt/live/x/cert.pem": "/etc/letsencrypt/live/x/cert.pem",
		"/etc/slipgate/tunnels/ab/key.pem": "/etc/slipgate/tunnels/ab/key.pem",
		"":                                 "",
	}
	for in, want := range cases {
		if got := movedPath(in, old, nw); got != want {
			t.Fatalf("movedPath(%q) = %q, want %q", in, got, want)
		}
	}
}
