package vectors

// Export internal functions for testing only.

func ExportSimilarity(a, b []string) float64 {
	return similarity(a, b)
}

func ExportMergeTokens(existing, incoming []string) []string {
	return mergeTokens(existing, incoming)
}

func ExportMaskParameters(tokens []string) []string {
	return maskParameters(tokens)
}

func ExportIsParameter(token string) bool {
	return isParameter(token)
}

func ExportTokenize(message string) []string {
	return tokenize(message)
}

func ExportReconstruct(tokens []string) string {
	return reconstruct(tokens)
}
