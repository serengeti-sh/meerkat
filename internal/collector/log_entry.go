package collector

import (
	"time"
)

// LogEntry is a normalized log record for embedding and storage.
type LogEntry struct {
	ID         string
	Timestamp  time.Time
	Service    string
	Severity   string
	Body       string
	Attributes map[string]string
}
