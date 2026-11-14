package handlers

import (
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"net/http"
	"net/http/httptest"
	"testing"
)

func useLocalPublicationTLS(t *testing.T) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.FileServer(http.Dir(transport.SubscriptionDir)).ServeHTTP(w, r)
	}))
	old := probePublished
	probePublished = func(host, token string) error { return probeAliases(srv.Client(), srv.URL, token) }
	t.Cleanup(func() { probePublished = old; srv.Close() })
}
