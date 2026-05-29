package server

import (
	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/tool"
	"github.com/serengeti-sh/meerkat/internal/vectorsclient"
)

// Export internal functions for testing only.

func ExportNewVectorsClient(cfg *config.Config) (vectorsclient.Client, error) {
	return newVectorsClient(cfg)
}

func ExportBuildToolRegistry(cfg *config.Config, vectorsClient vectorsclient.Client) (*tool.Registry, error) {
	return buildToolRegistry(cfg, vectorsClient)
}

func ExportBuildAnalyzerService(provider analyzer.LLMProvider, registry *tool.Registry, cfg *config.Config) (analyzer.Service, error) {
	return buildAnalyzerService(provider, registry, cfg)
}

func ExportBuildDatasourceRefs(cfg *config.Config) func() []analyzer.DatasourceRef {
	return buildDatasourceRefs(cfg)
}
