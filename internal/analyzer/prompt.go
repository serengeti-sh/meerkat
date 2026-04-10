package analyzer

import (
	"os"
	"strings"
)

const defaultSystemPrompt = `You are an SRE AI agent analyzing observability data for anomalies and issues.

You have access to tools that query metrics and logs from observability data sources.
Use them to gather data, then analyze it for:
1. Anomalies, error spikes, latency increases, resource exhaustion
2. Root cause hypotheses based on correlating metrics and logs
3. Severity assessment: info, warning, or critical
4. Recommended actions

When you have enough data to reach a conclusion, respond with a JSON analysis:
{
  "severity": "info|warning|critical",
  "summary": "one-line summary",
  "detail": "detailed analysis with root cause and recommendations"
}
Do NOT include the JSON inside markdown code blocks. Respond with raw JSON only.`

// LoadSystemPrompt loads the system prompt from the given file path,
// falling back to the built-in default if the file is empty or unreadable.
func LoadSystemPrompt(customPath string) string {
	if customPath == "" {
		return defaultSystemPrompt
	}

	data, err := os.ReadFile(customPath)
	if err != nil {
		return defaultSystemPrompt
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return defaultSystemPrompt
	}

	return trimmed
}
