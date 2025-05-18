package configloader

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server-specific configurations.

// ServerConfig holds server-specific configurations.
type ServerConfig struct {
	Port            string `yaml:"port"`
	TrackedNetworks string `yaml:"tracked_networks"`
}

// DBConfig holds database-specific configurations.

// DBConfig holds database-specific configurations.
type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// LoggingConfig holds logging-specific configurations.

// LoggingConfig holds logging-specific configurations.
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// CoinGeckoConfig holds CoinGecko API specific configurations.

// CoinGeckoConfig holds CoinGecko API specific configurations.
type CoinGeckoConfig struct {
	APIKey          string `yaml:"apiKey"`
	CacheTTLSeconds int    `yaml:"cacheTTLSeconds"`
}

// PerformanceConfig holds performance-related configurations.

// PerformanceConfig holds performance-related configurations.
type PerformanceConfig struct {
	MaxConcurrentRoutines int `yaml:"max_concurrent_routines"`
}

// Config is the top-level configuration structure.

// Config is the top-level configuration structure.
type Config struct {
	Server                    ServerConfig      `yaml:"server"`
	Database                  DBConfig          `yaml:"database"`
	Logging                   LoggingConfig     `yaml:"logging"`
	CoinGecko                 CoinGeckoConfig   `yaml:"coingecko"`
	Performance               PerformanceConfig `yaml:"performance"`
	TrackedNetworkIdentifiers []string          `yaml:"tracked_networks"`
}

// Load reads the YAML configuration file from the given path and unmarshals it.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data from %s: %w", path, err)
	}

	// Default values for performance if not set
	if cfg.Performance.MaxConcurrentRoutines <= 0 {
		cfg.Performance.MaxConcurrentRoutines = 10 // Default to 10 if not specified or invalid
	}

	return &cfg, nil
}

// GetTrackedNetworkShortNames parses the TrackedNetworks string from server config
// and returns a slice of short names.
func GetTrackedNetworkShortNames(cfg *Config) []string {
	if cfg == nil || cfg.Server.TrackedNetworks == "" {
		return []string{}
	}
	return strings.Split(strings.ReplaceAll(cfg.Server.TrackedNetworks, " ", ""), ",")
}
