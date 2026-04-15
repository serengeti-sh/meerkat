package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// CustomTool calls an arbitrary HTTP endpoint configured by the user.
type CustomTool struct {
	name        string
	description string
	method      string
	baseURL     string
	params      json.RawMessage
	client      *http.Client
}

// NewCustomTool creates a tool backed by an arbitrary HTTP endpoint.
func NewCustomTool(name, description, method, baseURL, paramSchemaFile string, client *http.Client) (Tool, error) {
	if name == "" {
		return nil, fmt.Errorf("custom tool: name is required")
	}
	if description == "" {
		return nil, fmt.Errorf("custom tool %q: description is required", name)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("custom tool %q: url is required", name)
	}

	m := method
	if m == "" {
		m = http.MethodGet
	}

	var params json.RawMessage
	if paramSchemaFile != "" {
		data, err := os.ReadFile(paramSchemaFile)
		if err != nil {
			return nil, fmt.Errorf("custom tool %q: failed to read param schema file %q: %w", name, paramSchemaFile, err)
		}
		params = json.RawMessage(data)
	}

	return &CustomTool{
		name:        name,
		description: description,
		method:      m,
		baseURL:     baseURL,
		params:      params,
		client:      client,
	}, nil
}

func (t *CustomTool) Name() string { return t.name }

func (t *CustomTool) Description() string { return t.description }

func (t *CustomTool) Parameters() json.RawMessage {
	if t.params != nil {
		return t.params
	}
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

func (t *CustomTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	var req *http.Request
	var err error

	switch t.method {
	case http.MethodGet:
		req, err = t.buildGetRequest(ctx, params)
	default:
		req, err = t.buildPostRequest(ctx, params)
	}
	if err != nil {
		return "", err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("custom tool %q request failed: %w", t.name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("custom tool %q: failed to read response: %w", t.name, err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("custom tool %q returned status %d: %s", t.name, resp.StatusCode, string(body))
	}

	return string(body), nil
}

func (t *CustomTool) buildGetRequest(ctx context.Context, params map[string]any) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, fmt.Sprintf("%v", v))
	}
	req.URL.RawQuery = q.Encode()
	return req, nil
}

func (t *CustomTool) buildPostRequest(ctx context.Context, params map[string]any) (*http.Request, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, t.method, t.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

var _ Tool = (*CustomTool)(nil)
