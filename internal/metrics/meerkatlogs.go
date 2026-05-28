package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MeerkatLogs metrics.
var (
	IngestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "meerkatlogs_ingest_total",
		Help: "Total number of log entries ingested",
	}, []string{"status"})

	IngestDedupTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "meerkatlogs_ingest_deduplicated_total",
		Help: "Total number of log entries deduplicated",
	})

	SearchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "meerkatlogs_search_duration_seconds",
		Help:    "Duration of search requests in seconds",
		Buckets: prometheus.DefBuckets,
	})

	SearchTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "meerkatlogs_search_total",
		Help: "Total number of search requests",
	}, []string{"status"})

	EmbedDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "meerkatlogs_embed_duration_seconds",
		Help:    "Duration of embedding requests in seconds",
		Buckets: prometheus.DefBuckets,
	})
)
