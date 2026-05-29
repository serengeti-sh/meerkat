package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	valid := func() *Config {
		return &Config{
			HTTP:     HTTPConfig{Port: 8080},
			Store:    StoreConfig{Host: "localhost", Name: "meerkat", User: "meerkat"},
			Analyzer: AnalyzerConfig{Provider: "openai"},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{
			name:    "valid config",
			modify:  func(_ *Config) {},
			wantErr: "",
		},
		{
			name: "negative http port",
			modify: func(c *Config) {
				c.HTTP.Port = -1
			},
			wantErr: "http.port",
		},
		{
			name: "missing store host",
			modify: func(c *Config) {
				c.Store.Host = ""
			},
			wantErr: "store.host",
		},
		{
			name: "missing store name",
			modify: func(c *Config) {
				c.Store.Name = ""
			},
			wantErr: "store.name",
		},
		{
			name: "missing store user",
			modify: func(c *Config) {
				c.Store.User = ""
			},
			wantErr: "store.user",
		},
		{
			name: "invalid analyzer provider",
			modify: func(c *Config) {
				c.Analyzer.Provider = "invalid"
			},
			wantErr: "analyzer.provider",
		},
		{
			name: "invalid vector store driver",
			modify: func(c *Config) {
				c.VectorStore.Driver = "redis"
			},
			wantErr: "vector_store.driver",
		},
		{
			name: "negative vectors port",
			modify: func(c *Config) {
				c.Vectors.Enabled = true
				c.Vectors.Port = -1
			},
			wantErr: "vectors.port",
		},
		{
			name: "invalid filter_mode",
			modify: func(c *Config) {
				c.Vectors.Enabled = true
				c.Vectors.Address = ":50051"
				c.Vectors.FilterMode = "magic"
			},
			wantErr: "vectors.filter_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.modify(c)
			err := c.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
