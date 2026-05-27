package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Suite manages the end-to-end test environment by running the actual binary.
type Suite struct {
	t          *testing.T
	BaseURL    string
	HTTPClient *http.Client

	// Infrastructure
	pgContainer testcontainers.Container
	pgHost      string
	pgPort      int

	// Mock services (running in-process for convenience)
	mockOpenAI     *mockOpenAIServer
	mockPrometheus *mockPrometheusServer

	// Server under test
	serverCmd *exec.Cmd
	tmpDir    string
}

// NewSuite creates a new end-to-end test suite.
func NewSuite(t *testing.T) *Suite {
	return &Suite{
		t: t,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start initializes the full e2e test environment with the real binary.
func (s *Suite) Start(ctx context.Context) error {
	// 1. Create temp directory for config and binary
	s.tmpDir = s.t.TempDir()

	// 2. Start PostgreSQL container
	c, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("meerkat_e2e"),
		postgres.WithUsername("meerkat"),
		postgres.WithPassword("meerkat"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return fmt.Errorf("start postgres container: %w", err)
	}
	s.pgContainer = c

	pgHost, err := c.Host(ctx)
	if err != nil {
		return fmt.Errorf("get postgres host: %w", err)
	}
	pgPortNat, err := c.MappedPort(ctx, "5432")
	if err != nil {
		return fmt.Errorf("get postgres port: %w", err)
	}
	s.pgHost = pgHost
	s.pgPort = int(pgPortNat.Num())

	// 3. Start mock services
	s.mockOpenAI = newMockOpenAIServer()
	s.mockPrometheus = newMockPrometheusServer()

	// 4. Copy resources to temp dir so the binary can find schema files
	if err := copyDir(filepath.Join(repoRoot(s.t), "resources"), filepath.Join(s.tmpDir, "resources")); err != nil {
		return fmt.Errorf("copy resources: %w", err)
	}

	// 5. Build meerkat-server binary
	binaryPath := filepath.Join(s.tmpDir, "meerkat-server")
	buildCmd := exec.CommandContext(ctx, "go", "build",
		"-o", binaryPath,
		"./cmd/meerkat-server",
	)
	buildCmd.Dir = repoRoot(s.t)
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build meerkat-server: %w\n%s", err, out)
	}

	// 5. Write e2e config file
	configPath := filepath.Join(s.tmpDir, "config.yaml")
	configContent := fmt.Sprintf(`
app:
  name: meerkat
  env: e2e
  debug: true

http:
  host: 127.0.0.1
  port: 18080

store:
  driver: postgres
  host: %s
  port: %d
  name: meerkat_e2e
  user: meerkat
  password: meerkat
  sslmode: disable

tools:
  prometheus:
    - name: test-vm
      url: %s

analyzer:
  provider: openai
  url: %s
  api_key: test-key
  model: gpt-4o-mock
  max_iterations: 5
  max_tokens: 1024
  temperature: 0.3
  system_prompt_file: %s
  skills_file: /dev/null

scheduler:
  enabled: false

inspector:
  dedup_window: 5m
  queue_size: 100
  worker_count: 2

reporter:
  webhook_url: ""
  min_severity: warning
`, s.pgHost, s.pgPort, s.mockPrometheus.URL(), s.mockOpenAI.URL(), writeSystemPrompt(s.t, s.tmpDir))

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	// 6. Start meerkat-server binary
	s.serverCmd = exec.CommandContext(ctx, binaryPath, "analyzer", "serve", "--config", configPath)
	s.serverCmd.Dir = s.tmpDir
	s.serverCmd.Stdout = os.Stdout
	s.serverCmd.Stderr = os.Stderr
	if err := s.serverCmd.Start(); err != nil {
		return fmt.Errorf("start meerkat-server: %w", err)
	}

	// 7. Wait for server to be ready (poll health endpoint)
	s.BaseURL = s.waitForServer(ctx, 30*time.Second)
	if s.BaseURL == "" {
		return fmt.Errorf("server failed to start within timeout")
	}

	return nil
}

func (s *Suite) waitForServer(ctx context.Context, timeout time.Duration) string {
	url := "http://127.0.0.1:18080/v1/health"
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := s.HTTPClient.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return "http://127.0.0.1:18080"
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return ""
}

// Stop cleans up the test environment.
func (s *Suite) Stop() {
	if s.serverCmd != nil && s.serverCmd.Process != nil {
		_ = s.serverCmd.Process.Kill()
		_ = s.serverCmd.Wait()
	}
	if s.mockOpenAI != nil {
		s.mockOpenAI.Close()
	}
	if s.mockPrometheus != nil {
		s.mockPrometheus.Close()
	}
	if s.pgContainer != nil {
		_ = s.pgContainer.Terminate(context.Background())
	}
}

// Post sends a POST request with JSON body.
func (s *Suite) Post(path string, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, s.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.HTTPClient.Do(req)
}

// Get sends a GET request.
func (s *Suite) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, s.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return s.HTTPClient.Do(req)
}

// ReadJSON decodes the response body into v.
func (s *Suite) ReadJSON(resp *http.Response, v any) error {
	defer func() { _ = resp.Body.Close() }()
	return json.NewDecoder(resp.Body).Decode(v)
}

// ReadBody reads the response body as string.
func (s *Suite) ReadBody(resp *http.Response) string {
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// WaitForReportStatus polls until the report reaches the expected status or times out.
func (s *Suite) WaitForReportStatus(reportID string, expectedStatus string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := s.Get("/v1/reports/" + reportID)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		var result map[string]any
		if err := s.ReadJSON(resp, &result); err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if result["status"] == expectedStatus {
			return result, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for report %s to reach status %s", reportID, expectedStatus)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return dir
}

func writeSystemPrompt(t *testing.T, tmpDir string) string {
	t.Helper()
	path := filepath.Join(tmpDir, "system_prompt.txt")
	content := `You are a test AI agent analyzing observability data.

When calling tools, use the EXACT tool name as provided. Do NOT invent tool names.
If a tool call fails, do NOT guess. Report the failure honestly.
If ALL datasources fail, set severity to "info" for resolved alerts or "warning" for firing alerts.

Respond with JSON only:
{"severity": "info|warning|critical", "summary": "...", "detail": "..."}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// SetupSuite creates and starts a full end-to-end test suite with the real binary.
func SetupSuite(t *testing.T) *Suite {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	suite := NewSuite(t)
	require.NoError(t, suite.Start(ctx), "Failed to start e2e test suite")
	t.Cleanup(func() {
		cancel()
		suite.Stop()
	})

	return suite
}
