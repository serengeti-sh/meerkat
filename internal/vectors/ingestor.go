package vectors

import "context"

// Ingestor abstracts log ingestion from various sources.
// Implementations can receive logs via OTLP, Kafka, HTTP, file tail, etc.
type Ingestor interface {
	// Name returns the ingestor identifier.
	Name() string

	// Start begins receiving logs and forwarding them to the given Service.
	Start(ctx context.Context, svc Service) error

	// Stop gracefully shuts down the ingestor.
	Stop(ctx context.Context) error
}
