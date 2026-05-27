package inspector

import (
	"encoding/json"
	"regexp"
	"strings"
)

// servicePatterns matches common service name labels in alerts and logs.
var servicePatterns = []*regexp.Regexp{
	// key=value patterns (Grafana labels, Prometheus, Loki)
	regexp.MustCompile(`\b(?:service|app|job|namespace|service_name|serviceName)\s*=\s*"?([^"\s,;]+)"?`),
	// key: value patterns (YAML, JSON-like)
	regexp.MustCompile(`\b(?:service|app|job|namespace|service_name|serviceName)\s*:\s*"?([^"\s,;]+)"?`),
	// Grafana alert labels
	regexp.MustCompile(`\blabels?\.(?:service|app|job)\s*=\s*"?([^"\s,;]+)"?`),
}

// jsonServiceKeys are JSON keys that commonly identify a service.
var jsonServiceKeys = []string{
	"service",
	"service_name",
	"serviceName",
	"app",
	"app_name",
	"appName",
	"job",
	"namespace",
	"pod",
	"deployment",
	"service_id",
}

// extractServiceFromAlert attempts to find a service name in the alert, message,
// or JSON payload using multiple heuristics.
func extractServiceFromAlert(alert, message string, data json.RawMessage) string {
	// 1. Try text patterns in alert and message
	for _, text := range []string{alert, message} {
		if svc := extractFromText(text); svc != "" {
			return svc
		}
	}

	// 2. Try JSON data payload
	if len(data) > 0 {
		if svc := extractFromJSON(data); svc != "" {
			return svc
		}
	}

	// 3. Try to extract from message as a fallback (word after "service" in natural language)
	for _, text := range []string{alert, message} {
		if svc := extractHeuristic(text); svc != "" {
			return svc
		}
	}

	return ""
}

func extractFromText(text string) string {
	if text == "" {
		return ""
	}
	for _, re := range servicePatterns {
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			svc := strings.TrimSpace(matches[1])
			svc = strings.Trim(svc, `"'`)
			if svc != "" && svc != "null" {
				return svc
			}
		}
	}
	return ""
}

func extractFromJSON(data json.RawMessage) string {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}

	// Try direct keys
	for _, key := range jsonServiceKeys {
		if v, ok := payload[key]; ok {
			if s, ok := v.(string); ok && s != "" && s != "null" {
				return s
			}
		}
	}

	// Try nested "labels" or "commonLabels" (Grafana alert format)
	for _, nested := range []string{"labels", "commonLabels", "alert", "payload"} {
		if v, ok := payload[nested]; ok {
			if obj, ok := v.(map[string]any); ok {
				for _, key := range jsonServiceKeys {
					if sv, ok := obj[key]; ok {
						if s, ok := sv.(string); ok && s != "" && s != "null" {
							return s
						}
					}
				}
			}
		}
	}

	return ""
}

var stopWords = map[string]bool{
	"is": true, "are": true, "was": true, "were": true,
	"has": true, "have": true, "had": true,
	"the": true, "a": true, "an": true, "this": true, "that": true,
	"to": true, "for": true, "of": true, "in": true, "on": true,
	"and": true, "or": true, "with": true, "from": true,
	"null": true, "unknown": true, "nil": true,
}

// extractHeuristic is a last-resort fallback that looks for "service" followed by a word.
func extractHeuristic(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ToLower(text)
	idx := strings.Index(text, "service")
	if idx == -1 {
		return ""
	}

	// Check if "service" is preceded by a word ("X service" pattern)
	if idx > 0 {
		before := text[:idx]
		before = strings.TrimRight(before, " \t")
		if i := strings.LastIndexAny(before, " \t"); i != -1 {
			candidate := strings.TrimSpace(before[i+1:])
			if len(candidate) >= 2 && !stopWords[candidate] {
				return candidate
			}
		} else if len(before) >= 2 && !stopWords[before] {
			return strings.TrimSpace(before)
		}
	}

	// Check "service X" pattern — skip if followed by = or : (those are handled by regex)
	after := text[idx+len("service"):]
	after = strings.TrimLeft(after, " \t")
	if len(after) == 0 || after[0] == '=' || after[0] == ':' || after[0] == '"' {
		return ""
	}
	end := strings.IndexAny(after, " \t\n,;}")
	if end == -1 {
		end = len(after)
	}
	candidate := strings.TrimSpace(after[:end])
	candidate = strings.Trim(candidate, `="':`)
	if len(candidate) >= 2 && !stopWords[candidate] {
		return candidate
	}
	return ""
}
