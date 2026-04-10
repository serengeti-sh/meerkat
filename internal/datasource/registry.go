package datasource

import "fmt"

// Registry maps datasource names to Provider instances.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry from the given providers.
func NewRegistry(providers []Provider) *Registry {
	m := make(map[string]Provider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &Registry{providers: m}
}

// Get returns a provider by datasource name.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("datasource %q not found", name)
	}
	return p, nil
}

// All returns all registered providers.
func (r *Registry) All() []Provider {
	result := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}
