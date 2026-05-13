package vectorstore

import (
	"testing"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearchResults(t *testing.T) {
	ids := entity.NewColumnVarChar("id", []string{"log-1", "log-2"})
	body := entity.NewColumnVarChar("body", []string{"connection refused", "timeout exceeded"})
	service := entity.NewColumnVarChar("service", []string{"api", "worker"})
	severity := entity.NewColumnVarChar("severity", []string{"ERROR", "WARN"})
	timestamp := entity.NewColumnInt64("timestamp", []int64{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		time.Date(2024, 1, 1, 0, 1, 0, 0, time.UTC).UnixMilli(),
	})

	result := client.SearchResult{
		IDs:    ids,
		Scores: []float32{0.95, 0.85},
		Fields: []entity.Column{body, service, severity, timestamp},
	}

	results, err := parseSearchResults(result)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "log-1", results[0].ID)
	assert.InDelta(t, 0.95, results[0].Score, 0.001)
	assert.Equal(t, "connection refused", results[0].Body)
	assert.Equal(t, "api", results[0].Service)
	assert.Equal(t, "ERROR", results[0].Severity)
	assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UTC(), results[0].Timestamp.UTC())

	assert.Equal(t, "log-2", results[1].ID)
	assert.Equal(t, "timeout exceeded", results[1].Body)
}

func TestParseSearchResults_Empty(t *testing.T) {
	ids := entity.NewColumnVarChar("id", []string{})

	result := client.SearchResult{
		IDs:    ids,
		Scores: []float32{},
		Fields: []entity.Column{},
	}

	results, err := parseSearchResults(result)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestParseSearchResults_WrongIDType(t *testing.T) {
	ids := entity.NewColumnInt64("id", []int64{1})

	result := client.SearchResult{
		IDs:    ids,
		Scores: []float32{0.5},
		Fields: []entity.Column{},
	}

	_, err := parseSearchResults(result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected id column type")
}

func TestRecord_FieldMapping(t *testing.T) {
	now := time.Now()
	r := Record{
		ID:         "test-id",
		Vector:     []float32{0.1, 0.2, 0.3},
		Timestamp:  now,
		Service:    "my-service",
		Severity:   "INFO",
		Body:       "test body",
		Attributes: map[string]string{"key": "value"},
	}

	assert.Equal(t, "test-id", r.ID)
	assert.Equal(t, "my-service", r.Service)
	assert.Equal(t, "INFO", r.Severity)
	assert.Equal(t, "test body", r.Body)
	assert.Equal(t, "value", r.Attributes["key"])
}

func TestSearchResult_FieldMapping(t *testing.T) {
	now := time.Now()
	sr := SearchResult{
		ID:        "result-1",
		Score:     0.99,
		Body:      "found log",
		Service:   "svc",
		Severity:  "ERROR",
		Timestamp: now,
	}

	assert.Equal(t, "result-1", sr.ID)
	assert.InDelta(t, 0.99, sr.Score, 0.001)
	assert.Equal(t, "found log", sr.Body)
}
