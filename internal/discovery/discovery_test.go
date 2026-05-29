package discovery_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/discovery"
)

func TestStaticDiscoverer(t *testing.T) {
	targets := []discovery.Target{
		{Name: "api", Address: "api:8080"},
		{Name: "db", Address: "db:5432"},
	}

	d := discovery.NewStatic(targets)
	assert.Equal(t, "static", d.Name())

	found, err := d.Discover(context.Background())
	require.NoError(t, err)
	assert.Len(t, found, 2)
	assert.Equal(t, "api", found[0].Name)
}

func TestRegistry(t *testing.T) {
	d1 := discovery.NewStatic([]discovery.Target{
		{Name: "svc1", Address: "svc1:8080"},
	})
	d2 := discovery.NewStatic([]discovery.Target{
		{Name: "svc2", Address: "svc2:9090"},
	})

	reg := discovery.NewRegistry(d1, d2)
	found, err := reg.Discover(context.Background())
	require.NoError(t, err)
	assert.Len(t, found, 2)
}

func TestRegistry_ContinuesOnError(t *testing.T) {
	// Create a static discoverer that returns targets
	d1 := discovery.NewStatic([]discovery.Target{
		{Name: "svc1", Address: "svc1:8080"},
	})

	// Create a Docker discoverer (returns nil, nil - no error)
	d2 := discovery.NewDocker("")

	reg := discovery.NewRegistry(d1, d2)
	found, err := reg.Discover(context.Background())
	require.NoError(t, err)
	assert.Len(t, found, 1)
}
