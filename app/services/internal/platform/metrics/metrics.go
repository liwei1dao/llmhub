// Package metrics declares Prometheus metrics used across services.
// All metrics are prefixed with "llmhub_".
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestDuration records the duration of inbound HTTP requests.
var HTTPRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "llmhub_http_request_duration_seconds",
		Help:    "Inbound HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"service", "path", "status"},
)

// CapabilityCalls counts calls per capability and status.
var CapabilityCalls = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "llmhub_capability_calls_total",
		Help: "Total AI capability calls.",
	},
	[]string{"capability", "model", "status"},
)

// ProviderUpstreamDuration records upstream provider call durations.
var ProviderUpstreamDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "llmhub_provider_upstream_duration_seconds",
		Help:    "Upstream provider call duration in seconds.",
		Buckets: []float64{0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120},
	},
	[]string{"provider", "capability"},
)

// PoolAccountState exposes a gauge of account counts per state.
var PoolAccountState = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "llmhub_pool_account_state",
		Help: "Count of pool accounts per provider/tier/state.",
	},
	[]string{"provider", "tier", "state"},
)

// SchedulerPickFailures counts pick failures by reason.
var SchedulerPickFailures = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "llmhub_scheduler_pick_failures_total",
		Help: "Scheduler pick failures.",
	},
	[]string{"reason"},
)
