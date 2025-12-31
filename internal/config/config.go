package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the agent configuration
type Config struct {
	Agent    AgentConfig    `mapstructure:"agent"`
	Scanner  ScannerConfig  `mapstructure:"scanner"`
	Reporter ReporterConfig `mapstructure:"reporter"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// AgentConfig represents agent-specific configuration
type AgentConfig struct {
	ID        string `mapstructure:"id"`
	ServerURL string `mapstructure:"server_url"`
	APIToken  string `mapstructure:"api_token"`
}

// ScannerConfig represents scanner-specific configuration
type ScannerConfig struct {
	Workers      int      `mapstructure:"workers"`
	MaxDepth     int      `mapstructure:"max_depth"`
	BatchSizeMB  int      `mapstructure:"batch_size_mb"`
	ExcludePaths []string `mapstructure:"exclude_paths"`
	MinFileSize  int64    `mapstructure:"min_file_size"`
	MaxFileSize  int64    `mapstructure:"max_file_size"`
}

// ReporterConfig represents reporter-specific configuration
type ReporterConfig struct {
	Compression        string `mapstructure:"compression"`
	CompressionLevel   int    `mapstructure:"compression_level"`
	TimeoutSeconds     int    `mapstructure:"timeout_seconds"`
	MaxRetries         int    `mapstructure:"max_retries"`
	RetryDelaySeconds  int    `mapstructure:"retry_delay_seconds"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Environment variable overrides
	v.SetEnvPrefix("YEMOUNE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Override API token from environment if set
	if token := os.Getenv("YEMOUNE_API_TOKEN"); token != "" {
		v.Set("agent.api_token", token)
	}
	if serverURL := os.Getenv("YEMOUNE_SERVER_URL"); serverURL != "" {
		v.Set("agent.server_url", serverURL)
	}
	if logLevel := os.Getenv("YEMOUNE_LOG_LEVEL"); logLevel != "" {
		v.Set("logging.level", logLevel)
	}

	// Unmarshal into config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set hostname as agent ID if not specified
	if cfg.Agent.ID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get hostname: %w", err)
		}
		cfg.Agent.ID = hostname
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Agent defaults
	v.SetDefault("agent.id", "")
	v.SetDefault("agent.server_url", "http://localhost:8080")
	v.SetDefault("agent.api_token", "")

	// Scanner defaults
	v.SetDefault("scanner.workers", 10)
	v.SetDefault("scanner.max_depth", 0) // 0 = unlimited
	v.SetDefault("scanner.batch_size_mb", 1024)
	v.SetDefault("scanner.exclude_paths", []string{
		"/proc/*",
		"/sys/*",
		"/dev/*",
		"*.tmp",
	})
	v.SetDefault("scanner.min_file_size", 0)
	v.SetDefault("scanner.max_file_size", 0) // 0 = unlimited

	// Reporter defaults
	v.SetDefault("reporter.compression", "zstd")
	v.SetDefault("reporter.compression_level", 3)
	v.SetDefault("reporter.timeout_seconds", 60)
	v.SetDefault("reporter.max_retries", 3)
	v.SetDefault("reporter.retry_delay_seconds", 5)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Agent.ServerURL == "" {
		return fmt.Errorf("agent.server_url is required")
	}
	if c.Scanner.Workers < 1 {
		return fmt.Errorf("scanner.workers must be >= 1")
	}
	if c.Scanner.BatchSizeMB < 1 {
		return fmt.Errorf("scanner.batch_size_mb must be >= 1")
	}
	if c.Reporter.MaxRetries < 0 {
		return fmt.Errorf("reporter.max_retries must be >= 0")
	}
	return nil
}
