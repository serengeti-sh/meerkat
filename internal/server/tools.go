package server

import (
	"fmt"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/tool"
	"github.com/serengeti-sh/meerkat/internal/vectorsclient"
)

func buildToolRegistry(cfg *config.Config, vectorsClient vectorsclient.Client) (*tool.Registry, error) {
	var tools []tool.Plugin

	for _, pc := range cfg.Tools.Prometheus {
		t, err := buildPrometheusTool(pc, cfg)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}

	for _, vc := range cfg.Tools.VictoriaLogs {
		t, err := buildVictoriaLogsTool(vc, cfg)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}

	for _, lc := range cfg.Tools.Loki {
		t, err := buildLokiTool(lc, cfg)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}

	if vectorsClient != nil {
		tools = append(tools, tool.NewSearchLogsTool(vectorsClient))
	}

	return tool.NewRegistry(tools...), nil
}

func buildPrometheusTool(pc config.PrometheusToolConfig, cfg *config.Config) (tool.Plugin, error) {
	httpClient, err := tool.NewHTTPClient(pc.CAFile)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", pc.Name, err)
	}
	description := cfg.Tools.PrometheusDescription
	if description == "" {
		description = "Query metrics using PromQL. Returns time series data."
	}
	schemaFile := cfg.Tools.PrometheusParamSchemaFile
	if schemaFile == "" {
		schemaFile = "internal/tool/schemas/prometheus.json"
	}
	t, err := tool.NewPrometheusTool(pc.Name, description, schemaFile, pc.URL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", pc.Name, err)
	}
	return t, nil
}

func buildVictoriaLogsTool(vc config.VictoriaLogsToolConfig, cfg *config.Config) (tool.Plugin, error) {
	httpClient, err := tool.NewHTTPClient(vc.CAFile)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", vc.Name, err)
	}
	description := cfg.Tools.VictoriaLogsDescription
	if description == "" {
		description = "Query logs using LogsQL. Returns log entries."
	}
	schemaFile := cfg.Tools.VictoriaLogsParamSchemaFile
	if schemaFile == "" {
		schemaFile = "internal/tool/schemas/victorialogs.json"
	}
	t, err := tool.NewVictoriaLogsTool(vc.Name, description, schemaFile, vc.URL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", vc.Name, err)
	}
	return t, nil
}

func buildLokiTool(lc config.LokiToolConfig, cfg *config.Config) (tool.Plugin, error) {
	httpClient, err := tool.NewHTTPClient(lc.CAFile)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", lc.Name, err)
	}
	description := cfg.Tools.LokiDescription
	if description == "" {
		description = "Query logs using LogQL. Returns log entries with timestamps and labels."
	}
	schemaFile := cfg.Tools.LokiParamSchemaFile
	if schemaFile == "" {
		schemaFile = "internal/tool/schemas/loki.json"
	}
	t, err := tool.NewLokiTool(lc.Name, description, schemaFile, lc.URL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", lc.Name, err)
	}
	return t, nil
}

func buildDatasourceRefs(cfg *config.Config) func() []analyzer.DatasourceRef {
	return func() []analyzer.DatasourceRef {
		var refs []analyzer.DatasourceRef
		for _, pc := range cfg.Tools.Prometheus {
			refs = append(refs, analyzer.DatasourceRef{Name: pc.Name, Type: "prometheus"})
		}
		for _, vc := range cfg.Tools.VictoriaLogs {
			refs = append(refs, analyzer.DatasourceRef{Name: vc.Name, Type: "victoria-logs"})
		}
		for _, lc := range cfg.Tools.Loki {
			refs = append(refs, analyzer.DatasourceRef{Name: lc.Name, Type: "loki"})
		}
		return refs
	}
}
