package vectors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/serengeti-sh/meerkat/internal/vectors"
)

func TestSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected float64
	}{
		{"empty a", []string{}, []string{"a", "b"}, 0},
		{"empty b", []string{"a", "b"}, []string{}, 0},
		{"both empty", []string{}, []string{}, 0},
		{"identical", []string{"a", "b", "c"}, []string{"a", "b", "c"}, 1.0},
		{"partial match", []string{"a", "b", "c"}, []string{"a", "b", "d"}, 0.6666666666666666},
		{"different lengths", []string{"a", "b"}, []string{"a", "b", "c"}, 0.6666666666666666},
		{"with wildcard", []string{"a", "<*>", "c"}, []string{"a", "b", "c"}, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vectors.ExportSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestMergeTokens(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		incoming []string
		expected []string
	}{
		{
			name:     "identical",
			existing: []string{"a", "b", "c"},
			incoming: []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "different at position",
			existing: []string{"a", "b", "c"},
			incoming: []string{"a", "x", "c"},
			expected: []string{"a", "<*>", "c"},
		},
		{
			name:     "incoming longer",
			existing: []string{"a", "b"},
			incoming: []string{"a", "b", "c"},
			expected: []string{"a", "b", "<*>"},
		},
		{
			name:     "existing longer",
			existing: []string{"a", "b", "c"},
			incoming: []string{"a", "b"},
			expected: []string{"a", "b", "<*>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vectors.ExportMergeTokens(tt.existing, tt.incoming)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskParameters(t *testing.T) {
	tests := []struct {
		name     string
		tokens   []string
		expected []string
	}{
		{
			name:     "no parameters",
			tokens:   []string{"connection", "refused"},
			expected: []string{"connection", "refused"},
		},
		{
			name:     "integer",
			tokens:   []string{"user", "12345", "logged", "in"},
			expected: []string{"user", "<*>", "logged", "in"},
		},
		{
			name:     "float",
			tokens:   []string{"latency", "12.34", "ms"},
			expected: []string{"latency", "<*>", "ms"},
		},
		{
			name:     "hex id",
			tokens:   []string{"request", "a1b2c3d4"},
			expected: []string{"request", "<*>"},
		},
		{
			name:     "ip address",
			tokens:   []string{"from", "192.168.1.1"},
			expected: []string{"from", "<*>"},
		},
		{
			name:     "date",
			tokens:   []string{"on", "2024-01-15"},
			expected: []string{"on", "<*>"},
		},
		{
			name:     "boolean",
			tokens:   []string{"enabled", "true"},
			expected: []string{"enabled", "<*>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vectors.ExportMaskParameters(tt.tokens)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsParameter(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"12345", true},
		{"12.34", true},
		{"abc123", false},
		{"a1b2c3d4e5f6", true}, // hex (8+ chars)
		{"2024-01-15", true},
		{"12:34:56", true},
		{"true", true},
		{"FALSE", true},
		{"/path/to/file", true},
		{"user@example.com", true},
		{"192.168.1.1", true},
		{"hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			result := vectors.ExportIsParameter(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected []string
	}{
		{"empty", "", nil},
		{"simple", "hello world", []string{"hello", "world"}},
		{"with punctuation", "error: connection failed!", []string{"error", "connection", "failed"}},
		{"with brackets", "[ERROR] connection refused", []string{"ERROR", "connection", "refused"}},
		{"multiple spaces", "hello   world", []string{"hello", "world"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vectors.ExportTokenize(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReconstruct(t *testing.T) {
	result := vectors.ExportReconstruct([]string{"hello", "world"})
	assert.Equal(t, "hello world", result)
}

func TestExtractor_CapacityLimit(t *testing.T) {
	d := vectors.NewExtractor()

	// Create maxTemplates + 1 unique templates
	for i := 0; i < 10001; i++ {
		d.Extract("message " + string(rune('a'+i%26)) + " " + string(rune('0'+i%10)))
	}

	// Should still be at max capacity
	templates := d.Templates()
	assert.LessOrEqual(t, len(templates), 10000)
}
