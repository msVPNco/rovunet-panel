package dnsrouter

import (
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/msVPNco/rovunet-panel/internal/config"
)

var errBudget = errors.New("DNS router resource budget exhausted")

const (
	defaultUDPWorkers         = 32
	defaultUDPQueue           = 256
	defaultMaxPending         = 8192
	defaultBackendPending     = 2048
	defaultBackendConnections = 32
	defaultResponseBytes      = 16 << 20
)

type resourceBudget struct {
	queryRate, responseRate                         tokenBudget
	queryRateDrops, responseRateDrops               atomic.Uint64
	workers, queue, maxBackendConnections           int
	maxPending, maxBackendPending, maxResponseBytes int64
	pending, responseBytes                          atomic.Int64
	pendingPeak, responsePeak, workerPeak           atomic.Int64
	workersBusy                                     atomic.Int64
	queueDrops, budgetDrops                         atomic.Uint64
}

// ResourceStats contains aggregate counters, never query names or client IPs.
type ResourceStats struct {
	QueryRateDrops     uint64 `json:"query_rate_drops"`
	ResponseRateDrops  uint64 `json:"response_rate_drops"`
	Pending            int64  `json:"pending"`
	PendingPeak        int64  `json:"pending_peak"`
	ResponseBytes      int64  `json:"response_bytes"`
	ResponsePeak       int64  `json:"response_peak"`
	WorkerPeak         int64  `json:"worker_peak"`
	QueueDrops         uint64 `json:"queue_drops"`
	BudgetDrops        uint64 `json:"budget_drops"`
	BackendConnections int    `json:"backend_connections"`
}

func (r *Router) Stats() ResourceStats {
	r.backendsMu.Lock()
	n := len(r.allBackends)
	r.backendsMu.Unlock()
	return ResourceStats{QueryRateDrops: r.queryRateDrops.Load(), ResponseRateDrops: r.responseRateDrops.Load(), Pending: r.pending.Load(), PendingPeak: r.pendingPeak.Load(), ResponseBytes: r.responseBytes.Load(), ResponsePeak: r.responsePeak.Load(), WorkerPeak: r.workerPeak.Load(), QueueDrops: r.queueDrops.Load(), BudgetDrops: r.budgetDrops.Load(), BackendConnections: n}
}

// ConfigureLimits is called before serving traffic. Existing configs use zeros.
func (r *Router) ConfigureLimits(l config.RouterLimits) error {
	if err := l.Validate(); err != nil {
		return err
	}
	r.workers, r.queue, r.maxPending, r.maxBackendPending, r.maxBackendConnections, r.maxResponseBytes = defaultUDPWorkers, defaultUDPQueue, defaultMaxPending, defaultBackendPending, defaultBackendConnections, defaultResponseBytes
	if l.UDPWorkers > 0 {
		r.workers = l.UDPWorkers
	}
	if l.UDPQueue > 0 {
		r.queue = l.UDPQueue
	}
	if l.Pending > 0 {
		r.maxPending = int64(l.Pending)
	}
	if l.BackendPending > 0 {
		r.maxBackendPending = int64(l.BackendPending)
	}
	if l.BackendConnections > 0 {
		r.maxBackendConnections = l.BackendConnections
	}
	if l.ResponseMiB > 0 {
		r.maxResponseBytes = int64(l.ResponseMiB) << 20
	}
	qps, burst, responseRate, responseBurst := 20000, 40000, 32, 64
	if l.UDPQueriesPerSecond > 0 {
		qps = l.UDPQueriesPerSecond
	}
	if l.UDPQueryBurst > 0 {
		burst = l.UDPQueryBurst
	}
	if l.UDPResponseMiBPerSecond > 0 {
		responseRate = l.UDPResponseMiBPerSecond
	}
	if l.UDPResponseBurstMiB > 0 {
		responseBurst = l.UDPResponseBurstMiB
	}
	r.queryRate.configure(float64(qps), float64(burst))
	r.responseRate.configure(float64(responseRate<<20), float64(responseBurst<<20))
	return nil
}

// Aggregate capacity only: no map keyed by a client/resolver IP is kept.
type tokenBudget struct {
	mu                  sync.Mutex
	rate, burst, tokens float64
	updated             time.Time
}

func (b *tokenBudget) configure(rate, burst float64) {
	b.rate = rate
	b.burst = burst
	b.tokens = burst
	b.updated = time.Now()
}
func (b *tokenBudget) allow(amount int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.updated).Seconds() * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.updated = now
	if float64(amount) > b.tokens {
		return false
	}
	b.tokens -= float64(amount)
	return true
}

func reserve(counter *atomic.Int64, amount, max int64) bool {
	for {
		old := counter.Load()
		if amount > max-old {
			return false
		}
		if counter.CompareAndSwap(old, old+amount) {
			return true
		}
	}
}

func peak(counter *atomic.Int64, value int64) {
	for {
		old := counter.Load()
		if value <= old || counter.CompareAndSwap(old, value) {
			return
		}
	}
}

func (r *Router) releasePending(bc *backendConn) { bc.pendingCount.Add(-1); r.pending.Add(-1) }

// Close releases sockets and workers in an isolated test or controlled shutdown.
func (r *Router) Close() error {
	r.closeOnce.Do(func() {
		close(r.done)
		if r.conn != nil {
			r.conn.Close()
		}
		r.tcpMu.Lock()
		if r.tcpListener != nil {
			r.tcpListener.Close()
		}
		for conn := range r.tcpConns {
			conn.Close()
		}
		r.tcpMu.Unlock()
		r.backendsMu.Lock()
		for bc := range r.allBackends {
			bc.conn.Close()
		}
		r.backendsMu.Unlock()
	})
	return nil
}

func (r *Router) closed() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

type udpJob struct {
	packet []byte
	from   *net.UDPAddr
}

func (r *Router) reportBudgets() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var previous uint64
	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			s := r.Stats()
			dropped := s.QueueDrops + s.BudgetDrops + s.QueryRateDrops + s.ResponseRateDrops
			if dropped != previous {
				log.Printf("DNS resource limits: pending=%d peak=%d workers_peak=%d backend_connections=%d queue_drops=%d budget_drops=%d query_rate_drops=%d response_rate_drops=%d", s.Pending, s.PendingPeak, s.WorkerPeak, s.BackendConnections, s.QueueDrops, s.BudgetDrops, s.QueryRateDrops, s.ResponseRateDrops)
				previous = dropped
			}
		}
	}
}
