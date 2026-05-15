package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SyncOperations counts finished sync operations by terminal status string.
var SyncOperations = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "k8s_ui",
		Name:      "sync_operations_total",
		Help:      "Completed sync operations by result status.",
	},
	[]string{"result"},
)

// HTTPRequests counts API requests by method and route pattern.
var HTTPRequests = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "k8s_ui",
		Name:      "http_requests_total",
		Help:      "HTTP API requests.",
	},
	[]string{"method", "path"},
)
