package inspect

import "encoding/json"

// Export internal functions for testing only.

func ExportExtractServiceFromAlert(alert, message string, data json.RawMessage) string {
	return extractServiceFromAlert(alert, message, data)
}

func ExportExtractFromText(text string) string {
	return extractFromText(text)
}

func ExportExtractFromJSON(data json.RawMessage) string {
	return extractFromJSON(data)
}

func ExportExtractHeuristic(text string) string {
	return extractHeuristic(text)
}
