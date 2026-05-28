package tool

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileSchema reads and compiles a JSON Schema file.
// Returns the compiled schema and the raw schema bytes.
func compileSchema(path string) (*jsonschema.Schema, json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read schema file %q: %w", path, err)
	}

	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("failed to parse schema file %q: %w", path, err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(path, doc); err != nil {
		return nil, nil, fmt.Errorf("failed to add schema resource %q: %w", path, err)
	}

	schema, err := c.Compile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid schema %q: %w", path, err)
	}

	return schema, data, nil
}

// validateArgs validates JSON arguments against a compiled schema.
func validateArgs(schema *jsonschema.Schema, args json.RawMessage) error {
	if schema == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(args, &v); err != nil {
		return fmt.Errorf("invalid JSON arguments: %w", err)
	}
	if err := schema.Validate(v); err != nil {
		return fmt.Errorf("invalid parameters: %w", err)
	}
	return nil
}

// argsToQueryParams validates args against schema and converts them to url.Values.
// Defaults are applied for missing fields.
func argsToQueryParams(schema *jsonschema.Schema, args json.RawMessage, defaults url.Values) (url.Values, error) {
	if err := validateArgs(schema, args); err != nil {
		return nil, err
	}

	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	q := url.Values{}
	maps.Copy(q, defaults)
	for k, v := range params {
		q.Set(k, formatParam(v))
	}
	return q, nil
}

// formatParam converts a parameter value to string.
func formatParam(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case int, int64:
		return fmt.Sprintf("%d", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseTime parses a time string. Supports RFC3339, Unix timestamp, and relative duration.
func parseTime(s string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try Unix timestamp (seconds)
	if unixSec, err := parseFloat(s); err == nil {
		return time.Unix(int64(unixSec), 0), nil
	}
	// Try relative duration
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("invalid time format %q: expected RFC3339, Unix timestamp, or duration", s)
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
