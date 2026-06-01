package server

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/tool"
)

func buildAnalyzerService(provider analyzer.LLMProvider, registry *tool.Registry, cfg *config.Config, log zerolog.Logger) (analyzer.Service, error) {
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
	}, log)
	if err != nil {
		return nil, fmt.Errorf("create analyzer service: %w", err)
	}
	return svc, nil
}
