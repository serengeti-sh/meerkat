package config

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	App       AppConfig       `mapstructure:"app"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Store     StoreConfig     `mapstructure:"store"`
	Tools     ToolConfig      `mapstructure:"tools"`
	Analyzer  AnalyzerConfig  `mapstructure:"analyzer"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Inspector InspectorConfig `mapstructure:"inspector"`
	Reporter  ReporterConfig  `mapstructure:"reporter"`
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
	Prometheus   []PrometheusToolConfig   `mapstructure:"prometheus"`
	VictoriaLogs []VictoriaLogsToolConfig `mapstructure:"victoria_logs"`
	Loki         []LokiToolConfig         `mapstructure:"loki"`
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

type LokiToolConfig struct {
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

type SchedulerConfig struct {
	Enabled bool                 `mapstructure:"enabled"`
	Jobs    []SchedulerJobConfig `mapstructure:"jobs"`
}

type ReporterConfig struct {
	WebhookURL  string `mapstructure:"webhook_url"`
	MinSeverity string `mapstructure:"min_severity"` // info, warning, critical
}

type InspectorConfig struct {
	DedupWindow string `mapstructure:"dedup_window"` // e.g. "5m", "30m"
}

// DSN builds the database connection string from individual config fields.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Store.Host,
		c.Store.Port,
		c.Store.User,
		c.Store.Password,
		c.Store.Name,
		c.Store.SSLMode,
	)
}

// GetDedupWindow parses the dedup window duration, falling back to 5m.
func (c InspectorConfig) GetDedupWindow() time.Duration {
	if c.DedupWindow == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(c.DedupWindow)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.App.Env) == "development"
}
