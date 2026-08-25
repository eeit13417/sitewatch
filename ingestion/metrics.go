package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// See docs/observability-design.md for what each of these answers and why
// these four specifically.
var (
	messagesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sitewatch_ingestion_messages_received_total",
		Help: "MQTT telemetry messages dequeued for processing.",
	})
	messagesDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sitewatch_ingestion_messages_dropped_total",
		Help: "MQTT telemetry messages dropped because the job queue was full.",
	})
	processingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "sitewatch_ingestion_message_processing_duration_seconds",
		Help:    "Time to fully process one telemetry message (Mongo write, Postgres update, alert evaluation).",
		Buckets: prometheus.DefBuckets,
	})
	alertDecisionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sitewatch_ingestion_alert_decisions_total",
		Help: "Alert engine decisions applied, by action.",
	}, []string{"action"})
)

// newMetricsServer serves /healthz and /metrics only — ingestion has no
// other HTTP surface, everything else arrives over MQTT.
func newMetricsServer(registry *prometheus.Registry, port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	return &http.Server{
		Addr:    ":" + port,
		Handler: mux,
		// See api/main.go's ReadHeaderTimeout comment — same Slowloris
		// concern (gosec G112), same fix.
		ReadHeaderTimeout: 5 * time.Second,
	}
}
