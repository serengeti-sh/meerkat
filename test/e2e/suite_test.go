package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mandacode-labs/inspector/ent"
	"github.com/mandacode-labs/inspector/internal/config"
	"github.com/mandacode-labs/inspector/internal/store"
)

// Suite manages the e2e test environment.
type Suite struct {
	t          *testing.T
	Client     *ent.Client
	BaseURL    string
	HTTPClient *http.Client
	DSN        string
}

// NewSuite creates a new e2e test suite.
func NewSuite(t *testing.T) *Suite {
	return &Suite{
		t: t,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start initializes the test database using testcontainers and verifies connectivity.
func (s *Suite) Start(ctx context.Context) error {
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

	cfg := &config.Config{
		Store: config.StoreConfig{
			Driver: "postgres",
			Path:   dsn,
		},
	}

	client, err := store.NewEntClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	if err := store.Migrate(ctx, client); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	s.Client = client
	return nil
}

// Stop cleans up the test environment.
func (s *Suite) Stop() {
	if s.Client != nil {
		_ = s.Client.Close()
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
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var result map[string]any
		if err := s.ReadJSON(resp, &result); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if result["status"] == expectedStatus {
			return result, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for report %s to reach status %s", reportID, expectedStatus)
}

// SetupSuite creates and starts a test suite with testcontainers.
// Skips if running in short mode.
func SetupSuite(t *testing.T) *Suite {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	suite := NewSuite(t)
	require.NoError(t, suite.Start(ctx), "Failed to start test suite")
	t.Cleanup(func() { suite.Stop() })

	return suite
}
