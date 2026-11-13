package config

import "fmt"

// RouterLimits bounds shared resources, not people, devices, IPs or resolvers.
// Zero uses the documented server default; no existing config is rewritten.
type RouterLimits struct {
	UDPWorkers              int `json:"udp_workers,omitempty"`
	UDPQueue                int `json:"udp_queue,omitempty"`
	Pending                 int `json:"pending,omitempty"`
	BackendPending          int `json:"backend_pending,omitempty"`
	BackendConnections      int `json:"backend_connections,omitempty"`
	ResponseMiB             int `json:"response_mib,omitempty"`
	UDPQueriesPerSecond     int `json:"udp_queries_per_second,omitempty"`
	UDPQueryBurst           int `json:"udp_query_burst,omitempty"`
	UDPResponseMiBPerSecond int `json:"udp_response_mib_per_second,omitempty"`
	UDPResponseBurstMiB     int `json:"udp_response_burst_mib,omitempty"`
}

func (l RouterLimits) Validate() error {
	for _, v := range []struct {
		name       string
		value, max int
	}{
		{"udp_workers", l.UDPWorkers, 128}, {"udp_queue", l.UDPQueue, 4096},
		{"pending", l.Pending, 65536}, {"backend_pending", l.BackendPending, 65536},
		{"backend_connections", l.BackendConnections, 128}, {"response_mib", l.ResponseMiB, 64},
		{"udp_queries_per_second", l.UDPQueriesPerSecond, 200000}, {"udp_query_burst", l.UDPQueryBurst, 400000},
		{"udp_response_mib_per_second", l.UDPResponseMiBPerSecond, 256}, {"udp_response_burst_mib", l.UDPResponseBurstMiB, 512},
	} {
		if v.value < 0 || v.value > v.max {
			return fmt.Errorf("router_limits.%s must be between 0 and %d", v.name, v.max)
		}
	}
	return nil
}
