package collector_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/collector"
	"github.com/serengeti-sh/meerkat/internal/config"
)

func TestGRPCServer_StartStop(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			OTLPBindAddr: "127.0.0.1:0",
		},
	}

	batcher := &mockBatcher{}
	srv := collector.NewGRPCServer(cfg, batcher)

	err := srv.Start()
	require.NoError(t, err)

	srv.Stop()
}

func TestGRPCServer_Start_InvalidAddr(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{
			OTLPBindAddr: "invalid::address",
		},
	}

	batcher := &mockBatcher{}
	srv := collector.NewGRPCServer(cfg, batcher)

	err := srv.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listen")
}

type mockBatcher struct{}

func (b *mockBatcher) Add(entry collector.LogEntry) error { return nil }

var _ collector.LogSink = (*mockBatcher)(nil)
