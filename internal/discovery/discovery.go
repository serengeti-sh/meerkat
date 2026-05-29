// Package discovery provides auto-discovery of services for monitoring.
package discovery

import "context"

// Target represents a discovered service endpoint.
type Target struct {
	Name      string            // Service name
	Address   string            // Service address (host:port or URL)
	Namespace string            // Kubernetes namespace or logical grouping
	Labels    map[string]string // Metadata labels
}

// Discoverer finds services to monitor.
type Discoverer interface {
	// Discover returns a list of targets.
	Discover(ctx context.Context) ([]Target, error)
	// Name returns the discoverer name.
	Name() string
}

// Registry holds multiple discoverers.
type Registry struct {
	discoverers []Discoverer
}

// NewRegistry creates a registry with the given discoverers.
func NewRegistry(discoverers ...Discoverer) *Registry {
	return &Registry{discoverers: discoverers}
}

// Discover runs all discoverers and returns aggregated targets.
func (r *Registry) Discover(ctx context.Context) ([]Target, error) {
	var allTargets []Target
	for _, d := range r.discoverers {
		targets, err := d.Discover(ctx)
		if err != nil {
			// Log error but continue with other discoverers.
			continue
		}
		allTargets = append(allTargets, targets...)
	}
	return allTargets, nil
}
