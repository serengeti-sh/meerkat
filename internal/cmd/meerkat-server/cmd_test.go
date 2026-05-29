package meerkatserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meerkatserver "github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server"
)

func TestNewCmd(t *testing.T) {
	cmd := meerkatserver.NewCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "meerkat-server", cmd.Use)
	assert.Equal(t, "Meerkat AI agent server", cmd.Short)

	// Check subcommands are registered
	subcommands := cmd.Commands()
	require.Len(t, subcommands, 2)

	names := make([]string, len(subcommands))
	for i, sub := range subcommands {
		names[i] = sub.Name()
	}
	assert.Contains(t, names, "analyzer")
	assert.Contains(t, names, "vectors")
}
