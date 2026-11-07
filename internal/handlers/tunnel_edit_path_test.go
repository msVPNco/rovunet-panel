package handlers

import "testing"

func TestMovedPath(t *testing.T) {
	old, nw := "/etc/rovunet/tunnels/a", "/etc/rovunet/tunnels/b"
	cases := map[string]string{
		"/etc/rovunet/tunnels/a/cert.pem":  "/etc/rovunet/tunnels/b/cert.pem",
		"/etc/letsencrypt/live/x/cert.pem": "/etc/letsencrypt/live/x/cert.pem",
		"/etc/rovunet/tunnels/ab/key.pem":  "/etc/rovunet/tunnels/ab/key.pem",
		"":                                 "",
	}
	for in, want := range cases {
		if got := movedPath(in, old, nw); got != want {
			t.Fatalf("movedPath(%q) = %q, want %q", in, got, want)
		}
	}
}
