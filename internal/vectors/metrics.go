package vectors

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	IngestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vectors_ingest_total",
		Help: "Total number of ingested entries",
	}, []string{"status"})

	IngestDedupTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vectors_ingest_deduplicated_total",
		Help: "Total number of deduplicated entries",
	})

	SearchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "vectors_search_duration_seconds",
		Help: "Search latency in seconds",
	})

	SearchTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vectors_search_total",
		Help: "Total number of search requests",
	}, []string{"status"})
)
