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
	Tokens           []Token                 `yaml:"tokens"` // This might be deprecated if tokens are fully managed by network files
	PortfolioService PortfolioServiceConfig  `yaml:"portfolioService"`
	CoinGecko        CoinGeckoConfig         `yaml:"coinGecko"` // Deprecated
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
	Name          string `yaml:"name"`               // User-friendly name, e.g., "Ethereum Mainnet"
	DEXScreenerID string `yaml:"dexScreenerChainID"` // Chain ID used by DEX Screener, e.g., "ethereum"
	Endpoint      string `yaml:"endpoint"`
	TokensFile    string `yaml:"tokensFile"` // Path to the JSON file containing token list for this network
	RPCTimeoutMs  int64  `yaml:"rpcTimeoutMs"`
	LimiterPeriod string `yaml:"limiterPeriod"` // e.g., "1s", "1m"
	LimiterBurst  int    `yaml:"limiterBurst"`
}

// Token represents a token that can be tracked. This might be deprecated.
// TODO: Review if this top-level Token config is still needed or if all tokens are defined per network.
type Token struct {
	Address string `yaml:"address"`
	Symbol  string `yaml:"symbol"`
	ChainID int64  `yaml:"chainID"` // To know which network this token belongs to if defined globally
}

// PortfolioServiceConfig holds configuration for the PortfolioService.
type PortfolioServiceConfig struct {
	UseMock                  bool `yaml:"useMock"`
	BalanceFetchTimeoutMs    int  `yaml:"balanceFetchTimeoutMs"`
	MaxConcurrentRequests    int  `yaml:"maxConcurrentRequests"`    // Max concurrent goroutines for fetching balances across all networks for a wallet
	MaxAddressesPerBatchCall int  `yaml:"maxAddressesPerBatchCall"` // Max addresses in a single batch call
}

// CoinGeckoConfig holds the configuration for the CoinGecko client.
// Deprecated: Will be removed or altered after DEXScreener integration.
// TODO: Review and remove or repurpose after DEXScreener integration is complete.
// AssetPlatformMapping might be useful for initial mapping to DEXScreener IDs if CoinGecko IDs are known.
type CoinGeckoConfig struct {
	BaseURL              string           `yaml:"baseURL"`
	ApiKey               string           `yaml:"apiKey"`
	RequestTimeoutMillis int64            `yaml:"requestTimeoutMillis"`
	AssetPlatformMapping map[int64]string `yaml:"assetPlatformMapping"` // Maps our chainID to CoinGecko platform ID
}

// DEXScreenerConfig holds the configuration for the DEX Screener client.
type DEXScreenerConfig struct {
	BaseURL              string `yaml:"baseURL"`
	RequestTimeoutMillis int64  `yaml:"requestTimeoutMillis"`
	// ChainMappings is removed; DEXScreenerChainID is now part of NetworkNode
}

// LoggingConfig holds the configuration for logging.
type LoggingConfig struct {
	Level string `yaml:"level"` // e.g., "debug", "info", "warn", "error"
	File  string `yaml:"file"`  // Optional: path to log file
}

// SwaggerConfig holds configuration for Swagger UI.
type SwaggerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"` // Path to serve Swagger UI, e.g., "/swagger"
}

// CacheConfig holds configuration for caching.
type CacheConfig struct {
	DefaultExpirationMinutes int `yaml:"defaultExpirationMinutes"`
	CleanupIntervalMinutes   int `yaml:"cleanupIntervalMinutes"`
}

// RpcClientConfig holds configuration for RPC clients.
type RpcClientConfig struct {
	DefaultTimeoutMs    int64 `yaml:"defaultTimeoutMs"`
	RateLimit           int   `yaml:"rateLimit"` // Requests per second
	BurstLimit          int   `yaml:"burstLimit"`
	MaxRetries          int   `yaml:"maxRetries"`
	RetryDelayMs        int64 `yaml:"retryDelayMs"`
	MaxIdleConnsPerHost int   `yaml:"maxIdleConnsPerHost"`
}

// TokenPriceServiceConfig holds configuration for the TokenPriceService.
type TokenPriceServiceConfig struct {
	RequestTimeoutMillis     int64 `yaml:"requestTimeoutMillis"`     // Timeout for a single request to the price provider
	CacheTTLMinutes          int   `yaml:"cacheTTLMinutes"`          // TTL for cached token prices
	MaxTokensPerBatchRequest int   `yaml:"maxTokensPerBatchRequest"` // Max tokens in a single batch request to price provider (e.g., 30 for DEXScreener)
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
