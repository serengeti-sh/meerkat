package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mandacode-labs/inspector/ent"
	"github.com/mandacode-labs/inspector/internal/analyzer"
	"github.com/mandacode-labs/inspector/internal/config"
	"github.com/mandacode-labs/inspector/internal/datasource"
	"github.com/mandacode-labs/inspector/internal/datasource/provider/prometheus"
	"github.com/mandacode-labs/inspector/internal/handler"
	"github.com/mandacode-labs/inspector/internal/inspector"
	insprepo "github.com/mandacode-labs/inspector/internal/inspector/repository"
	"github.com/mandacode-labs/inspector/internal/reporter"
	"github.com/mandacode-labs/inspector/internal/scheduler"
	"github.com/mandacode-labs/inspector/internal/store"
	"github.com/mandacode-labs/inspector/test/e2e/mock"

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
	server *http.Server
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
	// 1. Start PostgreSQL container
	c, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("inspector_test"),
		postgres.WithUsername("inspector"),
		postgres.WithPassword("inspector"),
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

	// 2. Start mock Prometheus
	s.MockPrometheus = mock.NewMockPrometheus()

	// 3. Start mock OpenAI
	s.MockOpenAI = mock.NewMockOpenAI()

	// 4. Build config with mock URLs
	cfg := &config.Config{
		App: config.AppConfig{
			Name: "inspector",
			Env:  "test",
		},
		HTTP: config.HTTPConfig{
			Host: "127.0.0.1",
			Port: 0, // random port
		},
		Store: config.StoreConfig{
			Driver: "postgres",
			Path:   dsn,
		},
		Datasources: []config.DatasourceConfig{
			{
				Name: "test-vm",
				Type: "victoria-metrics",
				URL:  s.MockPrometheus.URL(),
			},
		},
		Analyzer: config.AnalyzerConfig{
			Provider:      "openai",
			URL:           s.MockOpenAI.URL(),
			APIKey:        "test-key",
			Model:         "gpt-4o-mock",
			MaxIterations: 5,
			MaxTokens:     1024,
			Temperature:   0.3,
		},
		Scheduler: config.SchedulerConfig{
			Enabled: false,
		},
		Reporter: config.ReporterConfig{
			Channels: nil, // no reporter channels in tests
		},
	}

	// 5. Wire up dependencies (same as server.go but without fx)
	entClient, err := store.NewEntClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	if err := store.Migrate(ctx, entClient); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	s.Client = entClient

	// Datasource registry
	registry := datasource.NewRegistry([]datasource.Provider{
		prometheus.New("test-vm", s.MockPrometheus.URL()),
	})

	// Adapter for inspector.DatasourceRegistry
	dsRegistry := &adapterRegistry{Registry: registry}

	// Tool registry
	toolRegistry := analyzer.NewToolRegistry(
		inspector.NewQueryMetricsTool(registry),
		inspector.NewQueryLogsTool(registry),
	)

	// Analyzer
	llmProvider := analyzer.NewLLMProvider(analyzer.ProviderConfig{
		Provider:    cfg.Analyzer.Provider,
		URL:         cfg.Analyzer.URL,
		APIKey:      cfg.Analyzer.APIKey,
		Model:       cfg.Analyzer.Model,
		MaxTokens:   cfg.Analyzer.MaxTokens,
		Temperature: cfg.Analyzer.Temperature,
	})
	systemPrompt := analyzer.LoadSystemPrompt(cfg.Analyzer.SystemPromptFile)
	analyzerSvc := analyzer.NewService(llmProvider, toolRegistry, cfg.Analyzer.MaxIterations, systemPrompt)

	// Reporter (no-op in tests)
	reporterSvc := reporter.NewService(cfg)

	// Inspector service
	reportRepo := insprepo.NewRepository(entClient)
	inspectorSvc := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, dsRegistry)

	// Scheduler (disabled)
	sched := scheduler.NewCronScheduler(inspectorSvc, cfg)

	// HTTP handler
	h := handler.NewHandler(inspectorSvc, registry)

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
	for i := 0; i < 10; i++ {
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
}

// adapterRegistry adapts datasource.Registry to inspector.DatasourceRegistry.
type adapterRegistry struct {
	*datasource.Registry
}

func (a *adapterRegistry) All() []inspector.DatasourceRef {
	providers := a.Registry.All()
	refs := make([]inspector.DatasourceRef, 0, len(providers))
	for _, p := range providers {
		refs = append(refs, inspector.DatasourceRef{Name: p.Name(), Type: string(p.Type())})
	}
	return refs
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
