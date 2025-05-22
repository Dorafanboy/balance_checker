package configloader

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds server-specific configurations.
type ServerConfig struct {
	Port string `yaml:"port"`
}

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
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// CoinGeckoConfig holds CoinGecko API specific configurations.
type CoinGeckoConfig struct {
	APIKey               string            `yaml:"apiKey"`
	BaseURL              string            `yaml:"baseURL"`
	ClientTimeoutSeconds int               `yaml:"clientTimeoutSeconds"`
	VsCurrency           string            `yaml:"vsCurrency"`
	AssetPlatformMapping map[string]string `yaml:"assetPlatformMapping"`
}

// DEXScreenerConfig holds DEXScreener API specific configurations.
type DEXScreenerConfig struct {
	BaseURL              string `yaml:"baseURL"`
	RequestTimeoutMillis int64  `yaml:"requestTimeoutMillis"`
}

// TokenPriceServiceConfig holds configuration for the TokenPriceService.
type TokenPriceServiceConfig struct {
	MaxTokensPerBatchRequest int   `yaml:"maxTokensPerBatchRequest"`
	CacheTTLMinutes          int   `yaml:"cacheTTLMinutes"`
	RequestTimeoutMillis     int64 `yaml:"requestTimeoutMillis"`
}

// NetworkNodeConfig holds configuration for a specific blockchain network.
type NetworkNodeConfig struct {
	Name               string `yaml:"name"`
	RPCURL             string `yaml:"rpcURL"`
	DEXScreenerChainID string `yaml:"dexScreenerChainId"`
	ChainID            int64  `yaml:"chainID"`
}

// PerformanceConfig holds performance-related configurations.
type PerformanceConfig struct {
	MaxConcurrentRoutines int `yaml:"max_concurrent_routines"`
	RPCCallTimeoutSeconds int `yaml:"rpc_call_timeout_seconds"`
}

// Config is the top-level configuration structure.
type Config struct {
	Server        ServerConfig            `yaml:"server"`
	Database      DBConfig                `yaml:"database"`
	Logging       LoggingConfig           `yaml:"logging"`
	CoinGecko     CoinGeckoConfig         `yaml:"coingecko"`
	DEXScreener   DEXScreenerConfig       `yaml:"dexScreener"`
	TokenPriceSvc TokenPriceServiceConfig `yaml:"tokenPriceService"`
	Performance   PerformanceConfig       `yaml:"performance"`
	Networks      []NetworkNodeConfig     `yaml:"networks"`
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

	if cfg.Performance.MaxConcurrentRoutines <= 0 {
		cfg.Performance.MaxConcurrentRoutines = 10
	}
	if cfg.Performance.RPCCallTimeoutSeconds <= 0 {
		cfg.Performance.RPCCallTimeoutSeconds = 10
	}

	if cfg.CoinGecko.BaseURL == "" {
		cfg.CoinGecko.BaseURL = "https://api.coingecko.com/api/v3"
	}
	if cfg.CoinGecko.ClientTimeoutSeconds <= 0 {
		cfg.CoinGecko.ClientTimeoutSeconds = 10
	}
	if cfg.CoinGecko.VsCurrency == "" {
		cfg.CoinGecko.VsCurrency = "usd"
	}

	if cfg.DEXScreener.BaseURL == "" {
		cfg.DEXScreener.BaseURL = "https://api.dexscreener.com/api/v1"
	}
	if cfg.DEXScreener.RequestTimeoutMillis == 0 {
		cfg.DEXScreener.RequestTimeoutMillis = 10000
	}

	if cfg.TokenPriceSvc.MaxTokensPerBatchRequest == 0 {
		cfg.TokenPriceSvc.MaxTokensPerBatchRequest = 30
	}
	if cfg.TokenPriceSvc.CacheTTLMinutes == 0 {
		cfg.TokenPriceSvc.CacheTTLMinutes = 60
	}
	if cfg.TokenPriceSvc.RequestTimeoutMillis == 0 {
		if cfg.DEXScreener.RequestTimeoutMillis != 0 {
			cfg.TokenPriceSvc.RequestTimeoutMillis = cfg.DEXScreener.RequestTimeoutMillis
		} else {
			cfg.TokenPriceSvc.RequestTimeoutMillis = 10000
		}
	}

	return &cfg, nil
}
