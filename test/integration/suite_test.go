package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/ent"
	"github.com/serengeti-sh/meerkat/internal/httphandler"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	"github.com/serengeti-sh/meerkat/internal/reporter"
	"github.com/serengeti-sh/meerkat/internal/scheduler"
	"github.com/serengeti-sh/meerkat/internal/tool"
	"github.com/serengeti-sh/meerkat/test/integration/mock"

	_ "github.com/lib/pq"
)

// Suite manages the e2e test environment with mock services.
type Suite struct {
	t          *testing.T
	Client     *ent.Client
	BaseURL    string
	HTTPClient *http.Client
	DSN        string

	// Mock services
	MockPrometheus *mock.MockPrometheus
	MockOpenAI     *mock.MockOpenAI

	// Internal
	server           *http.Server
	systemPromptFile string
}

// NewSuite creates a new e2e test suite.
func NewSuite(t *testing.T) *Suite {
	return &Suite{
		t: t,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start initializes the full test environment
func (s *Suite) Start(ctx context.Context) error {
	// Create temporary system prompt file for tests
	tmpDir := os.TempDir()
	s.systemPromptFile = filepath.Join(tmpDir, fmt.Sprintf("meerkat-test-prompt-%d.txt", time.Now().UnixNano()))
	testPrompt := `You are a test AI agent analyzing observability data.

When calling tools, use the EXACT tool name as provided. Do NOT invent tool names.
If a tool call fails, do NOT guess. Report the failure honestly.
If ALL datasources fail, set severity to "info" for resolved alerts or "warning" for firing alerts.

Respond with JSON only:
{"severity": "info|warning|critical", "summary": "...", "detail": "..."}`
	if err := os.WriteFile(s.systemPromptFile, []byte(testPrompt), 0600); err != nil {
		return fmt.Errorf("create test system prompt file: %w", err)
	}

	// 1. Start PostgreSQL container
	c, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("meerkat_test"),
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

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("get connection string: %w", err)
	}
	s.DSN = dsn

	// Extract host and port from container for StoreConfig
	pgHost, err := c.Host(ctx)
	if err != nil {
		return fmt.Errorf("get postgres host: %w", err)
	}
	pgPortNat, err := c.MappedPort(ctx, "5432")
	if err != nil {
		return fmt.Errorf("get postgres port: %w", err)
	}
	pgPort := pgPortNat.Num()

	// 2. Start mock Prometheus
	s.MockPrometheus = mock.NewMockPrometheus()

	// 3. Start mock OpenAI
	s.MockOpenAI = mock.NewMockOpenAI()

	// 4. Build config with mock URLs
	cfg := &config.Config{
		App: config.AppConfig{
			Name: "meerkat",
			Env:  "test",
		},
		HTTP: config.HTTPConfig{
			Host: "127.0.0.1",
			Port: 0, // random port
		},
		Store: config.StoreConfig{
			Driver:   "postgres",
			Host:     pgHost,
			Port:     int(pgPort),
			Name:     "meerkat_test",
			User:     "meerkat",
			Password: "meerkat",
			SSLMode:  "disable",
		},
		Tools: config.ToolConfig{
			Prometheus: []config.PrometheusToolConfig{
				{Name: "test-vm", URL: s.MockPrometheus.URL()},
			},
		},
		Analyzer: config.AnalyzerConfig{
			Provider:         "openai",
			URL:              s.MockOpenAI.URL(),
			APIKey:           "test-key",
			Model:            "gpt-4o-mock",
			MaxIterations:    5,
			MaxTokens:        1024,
			Temperature:      0.3,
			SystemPromptFile: s.systemPromptFile,
		},
		Scheduler: config.SchedulerConfig{
			Enabled: false,
		},
		Reporter: config.ReporterConfig{},
	}

	// 5. Wire up dependencies (same as server.go but without fx)
	entClient, err := inspector.NewEntClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	if err := inspector.Migrate(ctx, entClient); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	s.Client = entClient

	// Tool registry
	promTool, err := tool.NewPrometheusTool("test-vm", "test prometheus", filepath.Join("..", "..", "internal", "tool", "schemas", "prometheus.json"), s.MockPrometheus.URL(), http.DefaultClient)
	if err != nil {
		return fmt.Errorf("create prometheus tool: %w", err)
	}
	toolRegistry := tool.NewRegistry(promTool)

	// Analyzer
	llmProvider := analyzer.NewLLMProvider(analyzer.ProviderConfig{
		Provider:    cfg.Analyzer.Provider,
		URL:         cfg.Analyzer.URL,
		APIKey:      cfg.Analyzer.APIKey,
		Model:       cfg.Analyzer.Model,
		MaxTokens:   cfg.Analyzer.MaxTokens,
		Temperature: cfg.Analyzer.Temperature,
	})
	systemPrompt, err := analyzer.LoadSystemPrompt(cfg.Analyzer.SystemPromptFile)
	if err != nil {
		return fmt.Errorf("load system prompt: %w", err)
	}
	analyzerSvc, err := analyzer.NewService(llmProvider, toolRegistry, analyzer.ServiceConfig{
		MaxIterations:       cfg.Analyzer.MaxIterations,
		SystemPrompt:        systemPrompt,
		MaxToolResultChars:  cfg.Analyzer.MaxToolResultChars,
		SummarizeOnOverflow: cfg.Analyzer.SummarizeOnOverflow,
		MaxContextMessages:  cfg.Analyzer.MaxContextMessages,
	})
	require.NoError(t, err)

	// Reporter (no-op in tests)
	reporterSvc := reporter.NewService(cfg.Reporter.WebhookURL, cfg.Reporter.MinSeverity, nil)

	// Inspector service
	reportRepo := inspector.NewEntReportRepository(entClient)
	dsRefs := func() []analyzer.DatasourceRef {
		return []analyzer.DatasourceRef{{Name: "test-vm", Type: "victoria-metrics"}}
	}
	inspectorSvc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, dsRefs, 5*time.Minute, 1000, 10)
	require.NoError(t, err)

	// Scheduler (disabled)
	sched := scheduler.NewService(inspectorSvc, cfg)

	// HTTP handler
	h, err := httphandler.New(inspectorSvc)
	require.NoError(t, err)

	// Start HTTP server on random port
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	s.BaseURL = fmt.Sprintf("http://%s", ln.Addr())

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	// Verify server is up
	for range 10 {
		resp, err := s.HTTPClient.Get(s.BaseURL + "/v1/health")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	_ = sched // scheduler disabled, not started
	return nil
}

// Stop cleans up the test environment.
func (s *Suite) Stop() {
	if s.server != nil {
		_ = s.server.Shutdown(context.Background())
	}
	if s.MockPrometheus != nil {
		s.MockPrometheus.Close()
	}
	if s.MockOpenAI != nil {
		s.MockOpenAI.Close()
	}
	if s.Client != nil {
		_ = s.Client.Close()
	}
	if s.systemPromptFile != "" {
		_ = os.Remove(s.systemPromptFile)
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

// SetupSuite creates and starts a full test suite with mock services.
func SetupSuite(t *testing.T) *Suite {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	suite := NewSuite(t)
	require.NoError(t, suite.Start(ctx), "Failed to start test suite")
	t.Cleanup(func() { suite.Stop() })

	return suite
}
