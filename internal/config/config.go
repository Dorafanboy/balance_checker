package config

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Config holds the overall configuration for the application.
type Config struct {
	Server           ServerConfig            `yaml:"server"`
	Networks         []NetworkNode           `yaml:"networks"`
	Tokens           []Token                 `yaml:"tokens"`
	PortfolioService PortfolioServiceConfig  `yaml:"portfolioService"`
	CoinGecko        CoinGeckoConfig         `yaml:"coinGecko"`
	DEXScreener      DEXScreenerConfig       `yaml:"dexScreener"`
	Logging          LoggingConfig           `yaml:"logging"`
	Swagger          SwaggerConfig           `yaml:"swagger"`
	Cache            CacheConfig             `yaml:"cache"`
	RpcClient        RpcClientConfig         `yaml:"rpcClient"`
	TokenPriceSvc    TokenPriceServiceConfig `yaml:"tokenPriceService"`
}

// ServerConfig holds the server-specific configuration.
type ServerConfig struct {
	Port         string `yaml:"port"`
	ReadTimeout  int    `yaml:"readTimeout"`
	WriteTimeout int    `yaml:"writeTimeout"`
	IdleTimeout  int    `yaml:"idleTimeout"`
}

// NetworkNode holds the configuration for a specific blockchain network node.
type NetworkNode struct {
	ChainID       int64  `yaml:"chainID"`
	Name          string `yaml:"name"`
	DEXScreenerID string `yaml:"dexScreenerChainID"`
	Endpoint      string `yaml:"endpoint"`
	TokensFile    string `yaml:"tokensFile"`
	RPCTimeoutMs  int64  `yaml:"rpcTimeoutMs"`
	LimiterPeriod string `yaml:"limiterPeriod"`
	LimiterBurst  int    `yaml:"limiterBurst"`
}

// Token represents a token that can be tracked. This might be deprecated.
type Token struct {
	Address string `yaml:"address"`
	Symbol  string `yaml:"symbol"`
	ChainID int64  `yaml:"chainID"`
}

// PortfolioServiceConfig holds configuration for the PortfolioService.
type PortfolioServiceConfig struct {
	UseMock                  bool `yaml:"useMock"`
	BalanceFetchTimeoutMs    int  `yaml:"balanceFetchTimeoutMs"`
	MaxConcurrentRequests    int  `yaml:"maxConcurrentRequests"`
	MaxAddressesPerBatchCall int  `yaml:"maxAddressesPerBatchCall"`
}

// CoinGeckoConfig holds the configuration for the CoinGecko client.
type CoinGeckoConfig struct {
	BaseURL              string           `yaml:"baseURL"`
	ApiKey               string           `yaml:"apiKey"`
	RequestTimeoutMillis int64            `yaml:"requestTimeoutMillis"`
	AssetPlatformMapping map[int64]string `yaml:"assetPlatformMapping"`
}

// DEXScreenerConfig holds the configuration for the DEX Screener client.
type DEXScreenerConfig struct {
	BaseURL              string `yaml:"baseURL"`
	RequestTimeoutMillis int64  `yaml:"requestTimeoutMillis"`
}

// LoggingConfig holds the configuration for logging.
type LoggingConfig struct {
	Level string `yaml:"level"` // e.g., "debug", "info", "warn", "error"
	File  string `yaml:"file"`
}

// SwaggerConfig holds configuration for Swagger UI.
type SwaggerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// CacheConfig holds configuration for caching.
type CacheConfig struct {
	DefaultExpirationMinutes int `yaml:"defaultExpirationMinutes"`
	CleanupIntervalMinutes   int `yaml:"cleanupIntervalMinutes"`
}

// RpcClientConfig holds configuration for RPC clients.
type RpcClientConfig struct {
	DefaultTimeoutMs    int64 `yaml:"defaultTimeoutMs"`
	RateLimit           int   `yaml:"rateLimit"`
	BurstLimit          int   `yaml:"burstLimit"`
	MaxRetries          int   `yaml:"maxRetries"`
	RetryDelayMs        int64 `yaml:"retryDelayMs"`
	MaxIdleConnsPerHost int   `yaml:"maxIdleConnsPerHost"`
}

// TokenPriceServiceConfig holds configuration for the TokenPriceService.
type TokenPriceServiceConfig struct {
	RequestTimeoutMillis     int64 `yaml:"requestTimeoutMillis"`
	CacheTTLMinutes          int   `yaml:"cacheTTLMinutes"`
	MaxTokensPerBatchRequest int   `yaml:"maxTokensPerBatchRequest"`
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	logrus.Infof("Loading configuration from path: %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		logrus.Errorf("Failed to read config file %s: %v", path, err)
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		logrus.Errorf("Failed to unmarshal config data from %s: %v", path, err)
		return nil, fmt.Errorf("failed to unmarshal config data from %s: %w", path, err)
	}

	// Apply default values for TokenPriceSvc if not set
	if cfg.TokenPriceSvc.MaxTokensPerBatchRequest == 0 {
		cfg.TokenPriceSvc.MaxTokensPerBatchRequest = 30 // Default for DEXScreener
		logrus.Infof("MaxTokensPerBatchRequest for TokenPriceSvc not set, defaulting to %d", cfg.TokenPriceSvc.MaxTokensPerBatchRequest)
	}
	if cfg.TokenPriceSvc.CacheTTLMinutes == 0 {
		cfg.TokenPriceSvc.CacheTTLMinutes = 60 // Default to 1 hour
		logrus.Infof("CacheTTLMinutes for TokenPriceSvc not set, defaulting to %d minutes", cfg.TokenPriceSvc.CacheTTLMinutes)
	}
	// Inherit RequestTimeoutMillis for TokenPriceSvc from DEXScreener if not set, or use a general default
	if cfg.TokenPriceSvc.RequestTimeoutMillis == 0 {
		if cfg.DEXScreener.RequestTimeoutMillis != 0 {
			cfg.TokenPriceSvc.RequestTimeoutMillis = cfg.DEXScreener.RequestTimeoutMillis
			logrus.Infof("TokenPriceSvc.RequestTimeoutMillis not set, defaulting to DEXScreener.RequestTimeoutMillis: %d ms", cfg.TokenPriceSvc.RequestTimeoutMillis)
		} else {
			cfg.TokenPriceSvc.RequestTimeoutMillis = 10000 // Default to 10 seconds if DEXScreener also not set
			logrus.Infof("TokenPriceSvc.RequestTimeoutMillis not set, defaulting to %d ms", cfg.TokenPriceSvc.RequestTimeoutMillis)
		}
	}

	// Apply default values for DEXScreener if not set
	if cfg.DEXScreener.BaseURL == "" {
		cfg.DEXScreener.BaseURL = "https://api.dexscreener.com"
		logrus.Infof("DEXScreener.BaseURL not set, defaulting to %s", cfg.DEXScreener.BaseURL)
	}
	if cfg.DEXScreener.RequestTimeoutMillis == 0 {
		cfg.DEXScreener.RequestTimeoutMillis = 10000 // Default to 10 seconds
		logrus.Infof("DEXScreener.RequestTimeoutMillis not set, defaulting to %d ms", cfg.DEXScreener.RequestTimeoutMillis)
	}

	// Validate that DEXScreenerChainID is set for each network
	for i, network := range cfg.Networks {
		if network.DEXScreenerID == "" {
			// Try to infer from CoinGecko mapping if available and if our ChainID matches
			if cgPlatformID, ok := cfg.CoinGecko.AssetPlatformMapping[network.ChainID]; ok && cgPlatformID != "" {
				// This is a simple heuristic, might need a more robust mapping
				// For now, we assume if a CoinGecko platform ID exists, it might be the same as DEXScreener's ID or a good hint.
				// This is a fallback and ideally should be explicitly configured.
				cfg.Networks[i].DEXScreenerID = cgPlatformID
				logrus.Warnf("Network '%s' (ChainID: %d) missing DEXScreenerChainID, attempting to use CoinGecko platform ID '%s'. Please configure DEXScreenerChainID explicitly.", network.Name, network.ChainID, cgPlatformID)
			} else {
				logrus.Warnf("Network '%s' (ChainID: %d) is missing DEXScreenerChainID in config. Price fetching for this network via DEXScreener might fail.", network.Name, network.ChainID)
				// Potentially return an error here or allow it to proceed with a warning
				// For now, just a warning.
			}
		}
	}

	logrus.Info("Configuration loaded successfully.")
	return &cfg, nil
}
