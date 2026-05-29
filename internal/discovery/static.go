package discovery

import "context"

// StaticDiscoverer returns a fixed list of targets from configuration.
type StaticDiscoverer struct {
	targets []Target
}

// NewStatic creates a static discoverer with the given targets.
func NewStatic(targets []Target) *StaticDiscoverer {
	return &StaticDiscoverer{targets: targets}
}

// Name returns the discoverer name.
func (s *StaticDiscoverer) Name() string {
	return "static"
}

// Discover returns the static targets.
func (s *StaticDiscoverer) Discover(ctx context.Context) ([]Target, error) {
	return s.targets, nil
}
