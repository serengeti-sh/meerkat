package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

var camelRe = regexp.MustCompile("([a-z0-9])([A-Z])")

// Load reads configuration from config.yaml in the current directory, env vars, and defaults.
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
			if err := normalizeKeys(settings); err != nil {
				return nil, fmt.Errorf("error normalizing config keys: %w", err)
			}
			for k, val := range settings {
				v.Set(k, val)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// LoadFromPath reads configuration from the specified file path, env vars, and defaults.
func LoadFromPath(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	bindEnvVars(v)

	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if settings := v.AllSettings(); len(settings) > 0 {
		if err := normalizeKeys(settings); err != nil {
			return nil, fmt.Errorf("error normalizing config keys: %w", err)
		}
		for k, val := range settings {
			v.Set(k, val)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func camelToSnake(s string) string {
	return strings.ToLower(camelRe.ReplaceAllString(s, "${1}_${2}"))
}

func normalizeKeys(m map[string]any) error {
	for key, value := range m {
		newKey := camelToSnake(key)
		if newKey != key {
			if existing, ok := m[newKey]; ok && existing != value {
				return fmt.Errorf("config key collision: %q and %q normalize to %q", key, findOriginalKey(m, newKey), newKey)
			}
			delete(m, key)
			m[newKey] = value
		}
		if nested, ok := value.(map[string]any); ok {
			if err := normalizeKeys(nested); err != nil {
				return err
			}
		} else if slice, ok := value.([]any); ok {
			for _, item := range slice {
				if nested, ok := item.(map[string]any); ok {
					if err := normalizeKeys(nested); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func findOriginalKey(m map[string]any, snakeKey string) string {
	for k := range m {
		if camelToSnake(k) == snakeKey && k != snakeKey {
			return k
		}
	}
	return ""
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
	v.SetDefault("store.port", 5432)
	v.SetDefault("store.sslmode", "disable")

	v.SetDefault("analyzer.provider", "openai")
	v.SetDefault("analyzer.url", "https://api.openai.com")
	v.SetDefault("analyzer.model", "gpt-4o")
	v.SetDefault("analyzer.max_iterations", 10)
	v.SetDefault("analyzer.max_tokens", 4096)
	v.SetDefault("analyzer.temperature", 0.3)
	v.SetDefault("analyzer.max_retries", 3)
	v.SetDefault("analyzer.retry_base_ms", 1000)
	v.SetDefault("analyzer.max_tool_result_chars", 30000)
	v.SetDefault("analyzer.summarize_on_overflow", true)
	v.SetDefault("analyzer.max_context_messages", 50)

	v.SetDefault("scheduler.enabled", false)

	v.SetDefault("inspect.queue_size", 1000)
	v.SetDefault("inspect.worker_count", 10)

	v.SetDefault("embed.provider", "openai")
	v.SetDefault("embed.model", "text-embedding-3-small")

	v.SetDefault("vector_store.milvus.database", "")
	v.SetDefault("vector_store.milvus.collection", "logs")
	v.SetDefault("vector_store.milvus.dimension", 1536)
	v.SetDefault("vector_store.milvus.retention", "72h")

	v.SetDefault("vectors.enabled", false)
	v.SetDefault("vectors.ingest_batch_size", 100)
	v.SetDefault("vectors.similarity_threshold", 0.8)
	v.SetDefault("vectors.max_context_logs", 50)
}

func bindEnvVars(v *viper.Viper) {
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("http.host", "HOST")
	_ = v.BindEnv("http.port", "PORT")
	_ = v.BindEnv("store.password", "DATABASE_PASSWORD")
	_ = v.BindEnv("analyzer.api_key", "LLM_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY")
	_ = v.BindEnv("analyzer.url", "LLM_URL")
	_ = v.BindEnv("reporter.webhook_url", "REPORTER_WEBHOOK_URL", "SLACK_WEBHOOK_URL")
	_ = v.BindEnv("reporter.min_severity", "REPORTER_MIN_SEVERITY")
	_ = v.BindEnv("http.tls.cert_file", "TLS_CERT_FILE")
	_ = v.BindEnv("http.tls.key_file", "TLS_KEY_FILE")
	_ = v.BindEnv("embedder.api_key", "EMBEDDER_API_KEY", "OPENAI_API_KEY")
	_ = v.BindEnv("vector_store.milvus.address", "MILVUS_ADDRESS")
	_ = v.BindEnv("vector_store.milvus.auth.user", "MILVUS_USER")
	_ = v.BindEnv("vector_store.milvus.auth.password", "MILVUS_PASSWORD")
	_ = v.BindEnv("vector_store.milvus.auth.token", "MILVUS_TOKEN")
	_ = v.BindEnv("vectors.address", "VECTORS_ADDRESS")
}
