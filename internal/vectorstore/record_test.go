package vectorstore_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

func TestNewRecord(t *testing.T) {
	now := time.Now()
	attrs := map[string]string{"key": "value"}

	r := vectorstore.NewRecord(
		[]float32{0.1, 0.2, 0.3},
		now,
		"api",
		"error",
		"connection refused",
		attrs,
	)

	assert.NotEmpty(t, r.ID)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, r.Vector)
	assert.Equal(t, now, r.Timestamp)
	assert.Equal(t, "api", r.Service)
	assert.Equal(t, "error", r.Severity)
	assert.Equal(t, "connection refused", r.Body)
	assert.Equal(t, attrs, r.Attributes)
}

func TestRecord_Validate(t *testing.T) {
	tests := []struct {
		name    string
		record  vectorstore.Record
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid record",
			record: vectorstore.Record{
				ID:     "1",
				Vector: []float32{0.1},
				Body:   "test",
			},
			wantErr: false,
		},
		{
			name: "missing id",
			record: vectorstore.Record{
				Vector: []float32{0.1},
				Body:   "test",
			},
			wantErr: true,
			errMsg:  "record ID is required",
		},
		{
			name: "missing vector",
			record: vectorstore.Record{
				ID:   "1",
				Body: "test",
			},
			wantErr: true,
			errMsg:  "record vector is required",
		},
		{
			name: "missing body",
			record: vectorstore.Record{
				ID:     "1",
				Vector: []float32{0.1},
			},
			wantErr: true,
			errMsg:  "record body is required",
		},
		{
			name:    "empty record",
			record:  vectorstore.Record{},
			wantErr: true,
			errMsg:  "record ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRecord_Validate_UniqueIDs(t *testing.T) {
	// Verify NewRecord generates unique IDs
	r1 := vectorstore.NewRecord([]float32{0.1}, time.Now(), "svc1", "info", "msg1", nil)
	r2 := vectorstore.NewRecord([]float32{0.2}, time.Now(), "svc2", "error", "msg2", nil)

	assert.NotEmpty(t, r1.ID)
	assert.NotEmpty(t, r2.ID)
	assert.NotEqual(t, r1.ID, r2.ID)
}
