package analyzer

import (
	"encoding/json"
	"time"
)

const responseTruncateLen = 500

func (s *service) parseFinalResponse(resp *CompletionResponse, iterations int) (*AnalysisResult, error) {
	content := resp.Content

	// Try to extract JSON from the response
	var parsed struct {
		Severity string `json:"severity"`
		Summary  string `json:"summary"`
		Detail   string `json:"detail"`
	}

	// Strip markdown code blocks if present
	content = stripCodeBlocks(content)

	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// If not valid JSON, use raw content as summary
		return &AnalysisResult{
			Severity:    SeverityInfo,
			Summary:     truncate(content, responseTruncateLen),
			Detail:      content,
			Iterations:  iterations,
			CompletedAt: time.Now(),
		}, nil
	}

	severity := Severity(parsed.Severity)
	switch severity {
	case SeverityWarning, SeverityCritical:
		// valid
	default:
		severity = SeverityInfo
	}

	return &AnalysisResult{
		Severity:    severity,
		Summary:     parsed.Summary,
		Detail:      parsed.Detail,
		Iterations:  iterations,
		CompletedAt: time.Now(),
	}, nil
}

func stripCodeBlocks(s string) string {
	if len(s) > 6 && s[:3] == "```" {
		// Find closing ```
		end := len(s) - 3
		if s[end:] == "```" {
			// Strip opening ```json or ``` and closing ```
			start := 3
			for start < len(s) && s[start] != '\n' {
				start++
			}
			if start < len(s) {
				start++ // skip newline
			}
			return s[start:end]
		}
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
