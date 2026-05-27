package rag

import (
	"regexp"
	"strings"
	"sync"
)

const (
	defaultSimilarityThreshold = 0.7
	maxTemplateLength          = 100
)

var (
	// tokenSplitter splits log messages into tokens.
	tokenSplitter = regexp.MustCompile(`[\s\[\]\(\)\{\}\<\>"',;:!?]+`)

	// paramPatterns matches common parameter patterns in logs.
	paramPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^\d+$`),                              // integers
		regexp.MustCompile(`^\d+\.\d+$`),                        // floats
		regexp.MustCompile(`^[0-9a-fA-F]{8,}$`),                  // hex IDs
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`),                // dates
		regexp.MustCompile(`^\d{2}:\d{2}:\d{2}`),                // times
		regexp.MustCompile(`^(true|false|TRUE|FALSE)$`),          // booleans
		regexp.MustCompile(`^/[^\s]*$`),                         // file paths
		regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`), // emails
		regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(?::\d+)?$`), // IP addresses
	}
)

// template represents a log template extracted by Extractor.
type template struct {
	tokens     []string
	count      int
	lastSeen   int64
	parameters []string
}

// Extractor extracts log templates using a simplified Drain algorithm.
// It groups similar log messages into templates, replacing parameters
// with wildcards.
type Extractor struct {
	mu         sync.RWMutex
	templates  []template
	threshold  float64
}

// NewExtractor creates a new Extractor instance with default settings.
func NewExtractor() *Extractor {
	return &Extractor{
		templates: make([]template, 0),
		threshold: defaultSimilarityThreshold,
	}
}

// Extract processes a log message and returns its template.
// If a matching template exists, it is updated; otherwise a new one is created.
func (d *Extractor) Extract(message string) (tmpl string, isNew bool) {
	tokens := tokenize(message)
	if len(tokens) == 0 {
		return message, true
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	bestIdx := -1
	bestScore := 0.0

	for i, tmpl := range d.templates {
		score := similarity(tokens, tmpl.tokens)
		if score > bestScore && score >= d.threshold {
			bestScore = score
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		// Update existing template
		tmpl := &d.templates[bestIdx]
		tmpl.tokens = mergeTokens(tmpl.tokens, tokens)
		tmpl.count++
		return reconstruct(tmpl.tokens), false
	}

	// Create new template
	tmplTokens := make([]string, len(tokens))
	copy(tmplTokens, tokens)
	tmplTokens = maskParameters(tmplTokens)

	d.templates = append(d.templates, template{
		tokens: tmplTokens,
		count:  1,
	})

	return reconstruct(tmplTokens), true
}

// Templates returns all extracted templates.
func (d *Extractor) Templates() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]string, 0, len(d.templates))
	for _, tmpl := range d.templates {
		result = append(result, reconstruct(tmpl.tokens))
	}
	return result
}

// Reset clears all extracted templates.
func (d *Extractor) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.templates = d.templates[:0]
}

// tokenize splits a log message into tokens.
func tokenize(message string) []string {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}

	parts := tokenSplitter.Split(message, -1)
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// similarity calculates the token-level similarity between two token slices.
func similarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 0.0
	}

	matches := 0
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] == b[i] || isWildcard(a[i]) || isWildcard(b[i]) {
			matches++
		}
	}

	return float64(matches) / float64(maxLen)
}

// mergeTokens combines two token sequences, creating wildcards where they differ.
func mergeTokens(existing, incoming []string) []string {
	maxLen := len(existing)
	if len(incoming) > maxLen {
		maxLen = len(incoming)
	}

	result := make([]string, maxLen)
	for i := 0; i < maxLen; i++ {
		if i >= len(existing) || i >= len(incoming) {
			result[i] = "<*>"
		} else if existing[i] == incoming[i] || isWildcard(existing[i]) {
			result[i] = existing[i]
		} else if isWildcard(incoming[i]) {
			result[i] = incoming[i]
		} else {
			result[i] = "<*>"
		}
	}

	return result
}

// maskParameters replaces parameter-like tokens with wildcards.
func maskParameters(tokens []string) []string {
	result := make([]string, len(tokens))
	for i, token := range tokens {
		if isParameter(token) {
			result[i] = "<*>"
		} else {
			result[i] = token
		}
	}
	return result
}

// isParameter checks if a token matches known parameter patterns.
func isParameter(token string) bool {
	for _, pattern := range paramPatterns {
		if pattern.MatchString(token) {
			return true
		}
	}
	return false
}

// isWildcard checks if a token is a wildcard.
func isWildcard(token string) bool {
	return token == "<*>"
}

// reconstruct rebuilds a log message from tokens.
func reconstruct(tokens []string) string {
	return strings.Join(tokens, " ")
}
