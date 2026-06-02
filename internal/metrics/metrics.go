package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coreapp_reconcile_total",
			Help: "Total number of reconciliations per CoreApp",
		},
		[]string{"name", "namespace", "result"},
	)

	AppReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "coreapp_ready",
			Help: "Indicates if the CoreApp is fully provisioned and ready (1) or not (0)",
		},
		[]string{"name", "namespace"},
	)

	ReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "coreapp_reconcile_duration_seconds",
			Help:    "Histogram of reconcile duration for CoreApp",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "namespace"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(ReconcileTotal, AppReady, ReconcileDuration)
}
