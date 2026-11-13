package dnsrouter

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"runtime"
	"sort"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

// This bounded loopback lab overloads only a test-owned backend. It makes no
// public resolver requests and does not assign an identity/limit to an IP.
func TestBoundedBurstAndSharedResolverRecovery(t *testing.T) {
	backend, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	var answering atomic.Bool
	go func() {
		buf := make([]byte, 4096)
		for {
			n, addr, err := backend.ReadFrom(buf)
			if err != nil {
				return
			}
			if answering.Load() {
				b := append([]byte(nil), buf[:n]...)
				b[2] |= 0x80
				backend.WriteTo(b, addr)
			}
		}
	}()
	var router *Router
	addr := startRouterWith(t, backend.LocalAddr().String(), func(r *Router) {
		router = r
		if err := r.ConfigureLimits(config.RouterLimits{UDPWorkers: 4, UDPQueue: 16, Pending: 64, BackendPending: 64, BackendConnections: 2, ResponseMiB: 1}); err != nil {
			t.Fatal(err)
		}
		r.pendingTTL = 200 * time.Millisecond
		r.cleanInterval = 20 * time.Millisecond
	})
	c, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var cpuBefore, cpuAfter syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &cpuBefore)
	const burst = 5000
	start := time.Now()
	for i := 0; i < burst; i++ {
		if _, err := c.Write(buildQuery(uint16(i), "x.t.example.com")); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for router.Stats().BudgetDrops+router.Stats().QueueDrops == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stats := router.Stats()
	if stats.PendingPeak > 64 || stats.WorkerPeak > 4 || stats.BackendConnections > 2 || stats.BudgetDrops+stats.QueueDrops == 0 {
		t.Fatalf("unbounded/untested overload: %+v", stats)
	}
	for router.Stats().Pending > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if router.Stats().Pending != 0 {
		t.Fatal("pending budget did not recover after blackhole timeout")
	}
	answering.Store(true)
	// A hundred distinct healthy TXIDs through the SAME source socket model
	// many clients behind one shared resolver after the bounded burst.
	var latencies []time.Duration
	var queryBytes, responseBytes int
	buf := make([]byte, 4096)
	for i := 0; i < 100; i++ {
		query := buildQuery(uint16(10000+i), "healthy.t.example.com")
		queryBytes += len(query)
		qstart := time.Now()
		c.SetDeadline(time.Now().Add(time.Second))
		c.Write(query)
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("healthy query %d failed: %v", i, err)
		}
		if binary.BigEndian.Uint16(buf[:2]) != uint16(10000+i) {
			t.Fatal("cross-client response mismatch")
		}
		responseBytes += n
		latencies = append(latencies, time.Since(qstart))
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	runtime.ReadMemStats(&after)
	syscall.Getrusage(syscall.RUSAGE_SELF, &cpuAfter)
	cpu := func(u syscall.Rusage) float64 {
		return float64(u.Utime.Sec+u.Stime.Sec) + float64(u.Utime.Usec+u.Stime.Usec)/1e6
	}
	report := map[string]any{"scenario": "synthetic loopback blackhole burst then shared-resolver recovery", "burst_attempts": burst, "healthy_attempts": 100, "healthy_failures": 0, "p50_us": latencies[49].Microseconds(), "p95_us": latencies[94].Microseconds(), "query_bytes": queryBytes, "response_bytes": responseBytes, "size_ratio": float64(responseBytes) / float64(queryBytes), "duration_ms": time.Since(start).Milliseconds(), "process_cpu_seconds": cpu(cpuAfter) - cpu(cpuBefore), "heap_live_before": before.HeapAlloc, "heap_live_after": after.HeapAlloc, "rss_process_peak_kib": cpuAfter.Maxrss, "stats": router.Stats(), "carrier_reachability": nil}
	b, _ := json.Marshal(report)
	t.Log(string(b))
}
