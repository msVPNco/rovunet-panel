package proxy

import "io"

// UsageSink receives the bytes a user moves. The serve commands set Meter
// before serving; it is nil (no counting) otherwise.
type UsageSink interface{ Add(user string, n int64) }

// Meter counts per-user traffic of the VLESS and SOCKS5 servers.
var Meter UsageSink

func meterAdd(user string, n int64) {
	if Meter != nil && user != "" {
		Meter.Add(user, n)
	}
}

type countReader struct {
	r    io.Reader
	user string
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	meterAdd(c.user, int64(n))
	return n, err
}

type countWriter struct {
	w    io.Writer
	user string
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	meterAdd(c.user, int64(n))
	return n, err
}
