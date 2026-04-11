package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App         AppConfig          `mapstructure:"app"`
	HTTP        HTTPConfig         `mapstructure:"http"`
	Store       StoreConfig        `mapstructure:"store"`
	Datasources []DatasourceConfig `mapstructure:"datasources"`
	Analyzer    AnalyzerConfig     `mapstructure:"analyzer"`
	Scheduler   SchedulerConfig    `mapstructure:"scheduler"`
	Reporter    ReporterConfig     `mapstructure:"reporter"`
}

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"`
	Debug   bool   `mapstructure:"debug"`
}

type HTTPConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	OpenAPIPath string `mapstructure:"openapi_path"`
}

type StoreConfig struct {
	Driver string `mapstructure:"driver"`
	Path   string `mapstructure:"path"` // PostgreSQL DSN, e.g. postgres://user:pass@localhost:5432/meerkat?sslmode=disable
}

type DatasourceConfig struct {
	Name  string         `mapstructure:"name"`
	Type  string         `mapstructure:"type"` // victoria-metrics, victoria-logs, prometheus, loki
	URL   string         `mapstructure:"url"`
	Extra map[string]any `mapstructure:"extra"` // provider-specific options
}

type AnalyzerConfig struct {
	Provider         string  `mapstructure:"provider"` // openai (default), anthropic
	URL              string  `mapstructure:"url"`
	APIKey           string  `mapstructure:"api_key"`
	Model            string  `mapstructure:"model"`
	MaxIterations    int     `mapstructure:"max_iterations"`
	MaxTokens        int     `mapstructure:"max_tokens"`
	Temperature      float64 `mapstructure:"temperature"`
	SystemPromptFile string  `mapstructure:"system_prompt_file"`
	SkillsFile       string  `mapstructure:"skills_file"`
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

type ReporterChannelConfig struct {
	Type        string `mapstructure:"type"` // slack, webhook, email
	WebhookURL  string `mapstructure:"webhook_url"`
	URL         string `mapstructure:"url"`
	MinSeverity string `mapstructure:"min_severity"` // info, warning, critical
}

type ReporterConfig struct {
	Channels []ReporterChannelConfig `mapstructure:"channels"`
}

var camelRe = regexp.MustCompile("([a-z0-9])([A-Z])")

func camelToSnake(s string) string {
	return strings.ToLower(camelRe.ReplaceAllString(s, "${1}_${2}"))
}

func normalizeKeys(m map[string]any) {
	for key, value := range m {
		newKey := camelToSnake(key)
		if newKey != key {
			delete(m, key)
			m[newKey] = value
		}
		if nested, ok := value.(map[string]any); ok {
			normalizeKeys(nested)
		} else if slice, ok := value.([]any); ok {
			for _, item := range slice {
				if nested, ok := item.(map[string]any); ok {
					normalizeKeys(nested)
				}
			}
		}
	}
}

func Load() (*Config, error) {
	v := viper.New()
	setDefaults(v)
	bindEnvVars(v)

	if _, err := os.Stat("config.yaml"); err == nil {
		v.SetConfigFile("config.yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config.yaml: %w", err)
		}
		if settings := v.AllSettings(); len(settings) > 0 {
			normalizeKeys(settings)
			for k, val := range settings {
				v.Set(k, val)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

func LoadFromPath(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	bindEnvVars(v)

	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if settings := v.AllSettings(); len(settings) > 0 {
		normalizeKeys(settings)
		for k, val := range settings {
			v.Set(k, val)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "meerkat")
	v.SetDefault("app.version", "0.0.1")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.debug", false)

	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.openapi_path", "api/openapi.yaml")

	v.SetDefault("store.driver", "postgres")
	v.SetDefault("store.path", "postgresql://localhost:5432/meerkat?sslmode=disable")

	v.SetDefault("analyzer.provider", "openai")
	v.SetDefault("analyzer.url", "https://api.openai.com")
	v.SetDefault("analyzer.model", "gpt-4o")
	v.SetDefault("analyzer.max_iterations", 10)
	v.SetDefault("analyzer.max_tokens", 4096)
	v.SetDefault("analyzer.temperature", 0.3)

	v.SetDefault("scheduler.enabled", false)
}

func bindEnvVars(v *viper.Viper) {
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("http.host", "HOST")
	_ = v.BindEnv("http.port", "PORT")
	_ = v.BindEnv("analyzer.api_key", "LLM_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY")
	_ = v.BindEnv("analyzer.url", "LLM_URL")
}

func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.App.Env) == "development"
}
