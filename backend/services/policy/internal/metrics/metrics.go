package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// CheckAccess metrics — per cache layer (L1/L2/L3).
var (
	CheckAccessTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ngac",
			Name:      "check_access_total",
			Help:      "Total checkAccess requests by cache layer hit.",
		},
		[]string{"layer"}, // "L1", "L2", "L3"
	)

	CheckAccessDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ngac",
			Name:      "check_access_duration_seconds",
			Help:      "CheckAccess latency by cache layer.",
			Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
		},
		[]string{"layer"},
	)
)

// Cache invalidation metrics.
var (
	CacheInvalidationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ngac",
			Name:      "cache_invalidation_total",
			Help:      "Cache invalidation events by scope (targeted or full).",
		},
		[]string{"scope"}, // "targeted", "full"
	)

	CacheKeysDeletedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "ngac",
			Name:      "cache_keys_deleted_total",
			Help:      "Total Redis cache keys deleted during invalidation.",
		},
	)
)

// Graph size metrics.
var (
	GraphNodeCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "ngac",
			Name:      "graph_node_count",
			Help:      "Number of nodes in the NGAC graph by type.",
		},
		[]string{"type"}, // "U", "UA", "OA", "PC", "O"
	)

	GraphAssociationCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "ngac",
			Name:      "graph_association_count",
			Help:      "Number of associations in the NGAC graph.",
		},
	)
)
