package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	App         AppConfig         `mapstructure:"app"`
	HTTP        HTTPConfig        `mapstructure:"http"`
	Store       StoreConfig       `mapstructure:"store"`
	Tools       ToolConfig        `mapstructure:"tools"`
	Analyzer    AnalyzerConfig    `mapstructure:"analyzer"`
	Schedule    ScheduleConfig    `mapstructure:"schedule"`
	Inspect     InspectConfig     `mapstructure:"inspect"`
	Notify      NotifyConfig      `mapstructure:"notify"`
	Embed       EmbedConfig       `mapstructure:"embed"`
	VectorStore VectorStoreConfig `mapstructure:"vector_store"`
	Vectors     VectorsConfig     `mapstructure:"vectors"`
}

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"`
	Debug   bool   `mapstructure:"debug"`
}

type TLSConfig struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type HTTPConfig struct {
	Host        string    `mapstructure:"host"`
	Port        int       `mapstructure:"port"`
	OpenAPIPath string    `mapstructure:"openapi_path"`
	TLS         TLSConfig `mapstructure:"tls"`
}

type StoreConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"sslmode"`
}

// ToolConfig groups tool configurations by type.
type ToolConfig struct {
	Prometheus                  []PrometheusToolConfig   `mapstructure:"prometheus"`
	PrometheusDescription       string                   `mapstructure:"prometheus_description"`
	PrometheusParamSchemaFile   string                   `mapstructure:"prometheus_param_schema_file"`
	VictoriaLogs                []VictoriaLogsToolConfig `mapstructure:"victoria_logs"`
	VictoriaLogsDescription     string                   `mapstructure:"victoria_logs_description"`
	VictoriaLogsParamSchemaFile string                   `mapstructure:"victoria_logs_param_schema_file"`
}

type PrometheusToolConfig struct {
	Name   string `mapstructure:"name"`
	URL    string `mapstructure:"url"`
	CAFile string `mapstructure:"ca_file"`
}

type VictoriaLogsToolConfig struct {
	Name   string `mapstructure:"name"`
	URL    string `mapstructure:"url"`
	CAFile string `mapstructure:"ca_file"`
}

type AnalyzerConfig struct {
	Provider            string  `mapstructure:"provider"` // openai (default), anthropic
	URL                 string  `mapstructure:"url"`
	APIKey              string  `mapstructure:"api_key"`
	Model               string  `mapstructure:"model"`
	MaxIterations       int     `mapstructure:"max_iterations"`
	MaxTokens           int     `mapstructure:"max_tokens"`
	Temperature         float64 `mapstructure:"temperature"`
	SystemPromptFile    string  `mapstructure:"system_prompt_file"`
	SkillsFile          string  `mapstructure:"skills_file"`
	MaxRetries          int     `mapstructure:"max_retries"`
	RetryBaseMs         int     `mapstructure:"retry_base_ms"`
	MaxToolResultChars  int     `mapstructure:"max_tool_result_chars"`
	SummarizeOnOverflow bool    `mapstructure:"summarize_on_overflow"`
	MaxContextMessages  int     `mapstructure:"max_context_messages"`
}

type SchedulerJobConfig struct {
	Name        string `mapstructure:"name"`
	Interval    string `mapstructure:"interval"`
	MetricQuery string `mapstructure:"metric_query"`
	LogQuery    string `mapstructure:"log_query"`
}

type ScheduleConfig struct {
	Enabled bool                 `mapstructure:"enabled"`
	Jobs    []SchedulerJobConfig `mapstructure:"jobs"`
}

type NotifyConfig struct {
	WebhookURL  string `mapstructure:"webhook_url"`
	MinSeverity string `mapstructure:"min_severity"` // info, warning, critical
}

type InspectConfig struct {
	DedupWindow string `mapstructure:"dedup_window"` // e.g. "5m", "30m"
	QueueSize   int    `mapstructure:"queue_size"`   // max queued analyses before rejecting
	WorkerCount int    `mapstructure:"worker_count"` // concurrent analysis workers
}

// DSN builds the database connection string from individual config fields.
func (c *Config) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", c.Store.Host, c.Store.Port),
		User:   url.UserPassword(c.Store.User, c.Store.Password),
		Path:   c.Store.Name,
	}
	q := u.Query()
	q.Set("sslmode", c.Store.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

// RedactedDSN returns the DSN with the password masked for logging.
func (c *Config) RedactedDSN() string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", c.Store.Host, c.Store.Port),
		User:   url.UserPassword(c.Store.User, "***REDACTED***"),
		Path:   c.Store.Name,
	}
	q := u.Query()
	q.Set("sslmode", c.Store.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

// GetDedupWindow parses the dedup window duration, falling back to 5m.
func (c InspectConfig) GetDedupWindow() time.Duration {
	if c.DedupWindow == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(c.DedupWindow)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

type EmbedConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"`
}

type VectorStoreConfig struct {
	Driver string       `mapstructure:"driver"` // milvus (default), qdrant
	Milvus MilvusConfig `mapstructure:"milvus"`
	Qdrant QdrantConfig `mapstructure:"qdrant"`
}

type QdrantConfig struct {
	Address    string `mapstructure:"address"`
	Collection string `mapstructure:"collection"`
	Dimension  int    `mapstructure:"dimension"`
	APIKey     string `mapstructure:"api_key"`
}

// VectorsConfig configures the vectors ingestion and search service.
type VectorsConfig struct {
	Enabled               bool          `mapstructure:"enabled"`
	Address               string        `mapstructure:"address"` // gRPC bind address
	Port                  int           `mapstructure:"port"`
	OTLPBindAddr          string        `mapstructure:"otlp_bind_addr"`
	IngestBatchSize       int           `mapstructure:"ingest_batch_size"`
	SimilarityThreshold   float64       `mapstructure:"similarity_threshold"`
	MaxContextLogs        int           `mapstructure:"max_context_logs"`
	FilterMode            string        `mapstructure:"filter_mode"`             // "all", "severity", "template"
	MinSeverity           string        `mapstructure:"min_severity"`            // info, warning, error, critical
	DeduplicateByTemplate bool          `mapstructure:"deduplicate_by_template"` // true (default), false
	Retention             time.Duration `mapstructure:"retention"`               // vector store TTL
}

func (c VectorsConfig) GetAddress() string {
	if c.Address != "" {
		return c.Address
	}
	if c.Port != 0 {
		return fmt.Sprintf(":%d", c.Port)
	}
	return ":50051"
}

func (c VectorsConfig) ShouldFilterBySeverity() bool {
	return c.FilterMode == "severity" && c.MinSeverity != "" && c.MinSeverity != "info"
}

func (c VectorsConfig) ShouldDeduplicate() bool {
	return c.FilterMode == "template" || c.DeduplicateByTemplate
}

type MilvusConfig struct {
	Address    string        `mapstructure:"address"`
	Database   string        `mapstructure:"database"`
	Collection string        `mapstructure:"collection"`
	Dimension  int           `mapstructure:"dimension"`
	Retention  time.Duration `mapstructure:"retention"`
	Auth       MilvusAuth    `mapstructure:"auth"`
	TLS        MilvusTLS     `mapstructure:"tls"`
}

type MilvusAuth struct {
	Enabled  bool   `mapstructure:"enabled"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Token    string `mapstructure:"token"`
}

type MilvusTLS struct {
	Enabled    bool   `mapstructure:"enabled"`
	CAFile     string `mapstructure:"ca_file"`
	SkipVerify bool   `mapstructure:"skip_verify"`
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.HTTP.Port < 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 0 and 65535")
	}

	if c.Store.Host == "" {
		return fmt.Errorf("store.host is required")
	}
	if c.Store.Name == "" {
		return fmt.Errorf("store.name is required")
	}
	if c.Store.User == "" {
		return fmt.Errorf("store.user is required")
	}

	if c.Analyzer.Provider != "" {
		switch c.Analyzer.Provider {
		case "openai", "anthropic":
			// ok
		default:
			return fmt.Errorf("analyzer.provider must be 'openai' or 'anthropic', got %q", c.Analyzer.Provider)
		}
	}

	if c.VectorStore.Driver != "" {
		switch c.VectorStore.Driver {
		case "milvus", "qdrant":
			// ok
		default:
			return fmt.Errorf("vector_store.driver must be 'milvus' or 'qdrant', got %q", c.VectorStore.Driver)
		}
	}

	if c.Vectors.Enabled {
		if c.Vectors.Port < 0 || c.Vectors.Port > 65535 {
			return fmt.Errorf("vectors.port must be between 0 and 65535")
		}
		if c.Vectors.Address == "" && c.Vectors.Port == 0 {
			return fmt.Errorf("vectors.address or vectors.port is required when vectors is enabled")
		}
		if c.Vectors.FilterMode != "" {
			switch c.Vectors.FilterMode {
			case "all", "severity", "template":
				// ok
			default:
				return fmt.Errorf("vectors.filter_mode must be 'all', 'severity', or 'template', got %q", c.Vectors.FilterMode)
			}
		}
	}

	return nil
}

func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.App.Env) == "development"
}
