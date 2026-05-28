package serve

import (
	"fmt"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/rag"
	"github.com/serengeti-sh/meerkat/internal/tool"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

// buildToolRegistry constructs the tool registry from configuration.
func buildToolRegistry(cfg *config.Config, emb embedder.Embedder, vstore vectorstore.Store) (*tool.Registry, error) {
	var tools []tool.Tool

	for _, pc := range cfg.Tools.Prometheus {
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
		tools = append(tools, t)
	}

	for _, lc := range cfg.Tools.Loki {
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
		tools = append(tools, t)
	}

	for _, vc := range cfg.Tools.VictoriaLogs {
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
		tools = append(tools, t)
	}

	for _, cc := range cfg.Tools.Custom {
		httpClient, err := tool.NewHTTPClient(cc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", cc.Name, err)
		}
		t, err := tool.NewCustomTool(cc.Name, cc.Description, cc.Method, cc.URL, cc.ParamSchemaFile, httpClient)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", cc.Name, err)
		}
		tools = append(tools, t)
	}

	if vstore != nil {
		tools = append(tools, tool.NewSearchLogsTool(emb, vstore))
	}

	if cfg.RAG.Enabled && vstore != nil {
		ragSvc, err := rag.NewService(emb, vstore)
		if err != nil {
			return nil, fmt.Errorf("create rag service: %w", err)
		}
		tools = append(tools, tool.NewSearchRAGTool(ragSvc))
	}

	return tool.NewRegistry(tools...), nil
}

// buildAnalyzerService constructs the analyzer service with loaded prompts and skills.
func buildAnalyzerService(provider analyzer.LLMProvider, registry *tool.Registry, cfg *config.Config) (analyzer.Service, error) {
	systemPrompt, err := analyzer.LoadSystemPrompt(cfg.Analyzer.SystemPromptFile)
	if err != nil {
		return nil, fmt.Errorf("load system prompt: %w", err)
	}
	skills, err := analyzer.LoadSkills(cfg.Analyzer.SkillsFile)
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}
	prompt := analyzer.MergeSkillsIntoPrompt(systemPrompt, skills)
	svc, err := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:       cfg.Analyzer.MaxIterations,
		SystemPrompt:        prompt,
		MaxToolResultChars:  cfg.Analyzer.MaxToolResultChars,
		SummarizeOnOverflow: cfg.Analyzer.SummarizeOnOverflow,
		MaxContextMessages:  cfg.Analyzer.MaxContextMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("create analyzer service: %w", err)
	}
	return svc, nil
}

// buildDatasourceRefs creates a function that returns configured datasource references.
func buildDatasourceRefs(cfg *config.Config) func() []analyzer.DatasourceRef {
	return func() []analyzer.DatasourceRef {
		var refs []analyzer.DatasourceRef
		for _, pc := range cfg.Tools.Prometheus {
			refs = append(refs, analyzer.DatasourceRef{Name: pc.Name, Type: "prometheus"})
		}
		for _, lc := range cfg.Tools.Loki {
			refs = append(refs, analyzer.DatasourceRef{Name: lc.Name, Type: "loki"})
		}
		for _, vc := range cfg.Tools.VictoriaLogs {
			refs = append(refs, analyzer.DatasourceRef{Name: vc.Name, Type: "victoria-logs"})
		}
		for _, cc := range cfg.Tools.Custom {
			refs = append(refs, analyzer.DatasourceRef{Name: cc.Name, Type: "custom"})
		}
		return refs
	}
}
