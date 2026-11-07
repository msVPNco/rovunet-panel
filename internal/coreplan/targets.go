package coreplan

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// RealityTargets is the bank REALITY picks its disguise from when none is
// given: large sites with heavy everyday traffic, so blocking them costs a
// censor more than blocking a VPN does. The list is a judgement, not a
// guarantee, and the server checks each one before use (TLS 1.3 and HTTP/2
// are required by REALITY). Edit it freely; entries are plain host names.
var RealityTargets = []string{
	"www.microsoft.com",
	"learn.microsoft.com",
	"www.amazon.com",
	"images-na.ssl-images-amazon.com",
	"m.media-amazon.com",
	"www.apple.com",
	"www.cloudflare.com",
	"dl.google.com",
	"www.googletagmanager.com",
	"fonts.gstatic.com",
	"www.nvidia.com",
	"www.samsung.com",
	"www.intel.com",
	"www.oracle.com",
	"addons.mozilla.org",
	"www.mozilla.org",
}

// CheckRealityTarget reports whether host:443 answers TLS 1.3 with HTTP/2
// from this server, which is what REALITY needs from its target.
func CheckRealityTarget(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config:    &tls.Config{ServerName: host, MinVersion: tls.VersionTLS13, NextProtos: []string{"h2", "http/1.1"}},
	}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return false
	}
	defer conn.Close()
	return conn.(*tls.Conn).ConnectionState().NegotiatedProtocol == "h2"
}

// PickRealityTarget returns a random host from targets that passes check.
// Candidates are tried in parallel; it fails when none passes (for example
// when the server cannot reach the internet).
func PickRealityTarget(targets []string, check func(string) bool, rnd *rand.Rand) (string, error) {
	ok := make([]bool, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t string) {
			defer wg.Done()
			ok[i] = check(t)
		}(i, t)
	}
	wg.Wait()
	var good []string
	for i, t := range targets {
		if ok[i] {
			good = append(good, t)
		}
	}
	if len(good) == 0 {
		return "", fmt.Errorf("none of the %d built-in REALITY targets answered TLS 1.3 with HTTP/2 from this server; check its internet access, or pass --target host:443", len(targets))
	}
	return good[rnd.Intn(len(good))], nil
}

// AutoReality fills in a REALITY core's target and server name when the
// operator gave none: a random working entry from RealityTargets. It does
// nothing for other protocols or when a target was given. It reports the host
// it chose, or "".
func AutoReality(o *Options) (string, error) {
	if o.Protocol != "reality" || strings.TrimSpace(o.RealityTarget) != "" {
		return "", nil
	}
	host, err := PickRealityTarget(RealityTargets, CheckRealityTarget, rand.New(rand.NewSource(time.Now().UnixNano())))
	if err != nil {
		return "", err
	}
	o.RealityTarget = host + ":443"
	if strings.TrimSpace(o.SNI) == "" {
		o.SNI = host
	}
	return host, nil
}
