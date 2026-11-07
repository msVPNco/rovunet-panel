package dnsrouter

import "testing"

// TestFindBackendPrefersLongestSuffixOnOverlap is the regression test for
// overlapping tunnel domains: findBackend used to return whichever
// registered route a map iteration visited first, which Go randomizes per
// run. With two tunnels sharing a suffix (a parent domain and a more
// specific subdomain both configured), a query for the subdomain could be
// routed to either tunnel's backend depending on iteration order that run.
// The more specific (longest) matching domain must always win, every time.
func TestFindBackendPrefersLongestSuffixOnOverlap(t *testing.T) {
	for i := 0; i < 50; i++ { // map iteration order varies run to run; repeat to catch nondeterminism
		r := New("127.0.0.1:0")
		r.AddRoute("t.example.com", "parent-backend:1")
		r.AddRoute("sub.t.example.com", "child-backend:1")

		if got := r.findBackend("sub.t.example.com."); got != "child-backend:1" {
			t.Fatalf("iteration %d: exact subdomain query got %q, want the more specific child-backend:1", i, got)
		}
		if got := r.findBackend("deep.sub.t.example.com."); got != "child-backend:1" {
			t.Fatalf("iteration %d: nested subdomain query got %q, want the more specific child-backend:1", i, got)
		}
		if got := r.findBackend("other.t.example.com."); got != "parent-backend:1" {
			t.Fatalf("iteration %d: a sibling subdomain that doesn't match the child route got %q, want parent-backend:1", i, got)
		}
		if got := r.findBackend("t.example.com."); got != "parent-backend:1" {
			t.Fatalf("iteration %d: the parent domain's own exact query got %q, want parent-backend:1", i, got)
		}
	}
}

// TestFindBackendFallsBackToDefault confirms the ordinary no-match path
// still works unchanged.
func TestFindBackendFallsBackToDefault(t *testing.T) {
	r := New("127.0.0.1:0")
	r.AddRoute("t.example.com", "backend:1")
	r.SetDefault("default-backend:1")

	if got := r.findBackend("unrelated.example.org."); got != "default-backend:1" {
		t.Fatalf("got %q, want default-backend:1", got)
	}
}
