package vectorstore

// Export internal functions for testing only.
// These are not part of the public API.

func JoinQuoted(ids []string) string {
	return joinQuoted(ids)
}

func NewAPIKeyAuth(key string) *apiKeyAuth {
	return &apiKeyAuth{key: key}
}
