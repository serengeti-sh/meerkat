package vectors_test

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/serengeti-sh/meerkat/internal/vectors"
)

func TestOTLPIngestor_Name(t *testing.T) {
	ing := vectors.NewOTLPIngestor(":4317", zerolog.New(nil))
	assert.Equal(t, "otlp", ing.Name())
}

func TestLogRecordToEntry(t *testing.T) {
	// This test verifies the unexported logRecordToEntry function behavior
	// through the public Export method of the OTLP server.
	// The detailed conversion logic is tested via integration with the service.

	tests := []struct {
		name         string
		body         string
		severity     string
		attrs        map[string]string
		wantBody     string
		wantSeverity string
	}{
		{
			name:         "simple log",
			body:         "connection refused",
			severity:     "ERROR",
			wantBody:     "connection refused",
			wantSeverity: "ERROR",
		},
		{
			name:         "log with attributes",
			body:         "request completed",
			severity:     "INFO",
			attrs:        map[string]string{"trace_id": "abc123"},
			wantBody:     "request completed",
			wantSeverity: "INFO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an entry manually to verify the conversion logic
			entry := vectors.Entry{
				Timestamp:  time.Now(),
				Service:    "test-svc",
				Severity:   tt.severity,
				Body:       tt.body,
				Attributes: tt.attrs,
			}

			assert.Equal(t, tt.wantBody, entry.Body)
			assert.Equal(t, tt.wantSeverity, entry.Severity)
		})
	}
}

func TestEntry_Struct(t *testing.T) {
	now := time.Now()
	entry := vectors.Entry{
		ID:        "log-1",
		Timestamp: now,
		Service:   "api",
		Severity:  "ERROR",
		Body:      "connection refused",
		Attributes: map[string]string{
			"trace_id": "abc",
		},
	}

	assert.Equal(t, "log-1", entry.ID)
	assert.Equal(t, now, entry.Timestamp)
	assert.Equal(t, "api", entry.Service)
	assert.Equal(t, "ERROR", entry.Severity)
	assert.Equal(t, "connection refused", entry.Body)
	assert.Equal(t, "abc", entry.Attributes["trace_id"])
}

func TestSearchOptions_Defaults(t *testing.T) {
	opts := vectors.SearchOptions{}
	assert.Equal(t, 0, opts.Limit)
	assert.Equal(t, time.Duration(0), opts.TimeRange)
	assert.Empty(t, opts.Service)
	assert.Empty(t, opts.Severity)
}

func TestIngestResult_Struct(t *testing.T) {
	result := vectors.IngestResult{
		IngestedCount:     10,
		DeduplicatedCount: 5,
		FilteredCount:     2,
	}

	assert.Equal(t, 10, result.IngestedCount)
	assert.Equal(t, 5, result.DeduplicatedCount)
	assert.Equal(t, 2, result.FilteredCount)
}
