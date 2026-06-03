/*
Copyright 2026 Juandi.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
