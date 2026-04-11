package analyzer

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrSystemPromptNotConfigured = errors.New("system prompt file path not configured")

// LoadSystemPrompt loads the system prompt from the given file path.
// Returns an error if the path is empty or the file cannot be read.
func LoadSystemPrompt(customPath string) (string, error) {
	if customPath == "" {
		return "", ErrSystemPromptNotConfigured
	}

	data, err := os.ReadFile(customPath)
	if err != nil {
		return "", fmt.Errorf("failed to read system prompt file %q: %w", customPath, err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", fmt.Errorf("system prompt file %q is empty", customPath)
	}

	return trimmed, nil
}

// MustLoadSystemPrompt loads the system prompt and panics on error.
// Use this during initialization to fail fast if the prompt is not configured.
func MustLoadSystemPrompt(customPath string) string {
	prompt, err := LoadSystemPrompt(customPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load system prompt: %v", err))
	}
	return prompt
}
