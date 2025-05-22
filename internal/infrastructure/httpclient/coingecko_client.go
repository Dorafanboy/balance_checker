package httpclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"balance_checker/internal/entity"
	"balance_checker/internal/infrastructure/configloader"
	// "balance_checker/internal/pkg/logger" // Будем использовать свой интерфейс

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
)

// Logger defines a simple logging interface that our client will use.
// This allows decoupling from a specific logger implementation.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// CoinGeckoClient defines the interface for interacting with the CoinGecko API.
type CoinGeckoClient interface {
	// GetTokenPricesByContractAddresses fetches prices for given token contract addresses on a specific platform.
	// Returns a map of contractAddress (lowercase) to price in the configured vsCurrency.
	GetTokenPricesByContractAddresses(ctx context.Context, platformID string, contractAddresses []string) (map[string]float64, error)
}

// DEXScreenerClient defines the interface for interacting with the DEX Screener API.
// TODO: Это временная заглушка. Нужно будет реализовать полноценный клиент или использовать существующий.
type DEXScreenerClient interface {
	GetTokenPairsByAddresses(ctx context.Context, dexscreenerChainID string, tokenAddresses []string) ([]entity.PairData, error)
}

// coinGeckoClientFastHTTP implements CoinGeckoClient using fasthttp.
type coinGeckoClientFastHTTP struct {
	client *fasthttp.Client
	cfg    configloader.CoinGeckoConfig
	logger Logger // MODIFIED: Use local Logger interface
	json   jsoniter.API
}

// NewCoinGeckoClient creates a new CoinGecko client.
func NewCoinGeckoClient(cfg configloader.CoinGeckoConfig, appLogger Logger) (CoinGeckoClient, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("CoinGecko baseURL is not configured")
	}
	if cfg.VsCurrency == "" {
		return nil, fmt.Errorf("CoinGecko vsCurrency is not configured")
	}

	timeout := time.Duration(cfg.ClientTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second // Default timeout if not configured or invalid
	}

	return &coinGeckoClientFastHTTP{
		client: &fasthttp.Client{
			ReadTimeout:         timeout,
			WriteTimeout:        timeout,
			MaxIdleConnDuration: time.Hour, // Can be configured further if needed
			// Other fasthttp client settings can be added here
		},
		cfg:    cfg,
		logger: appLogger,
		json:   jsoniter.ConfigCompatibleWithStandardLibrary,
	}, nil
}

// GetTokenPricesByContractAddresses fetches token prices from CoinGecko.
func (c *coinGeckoClientFastHTTP) GetTokenPricesByContractAddresses(
	ctx context.Context,
	platformID string,
	contractAddresses []string,
) (map[string]float64, error) {
	if len(contractAddresses) == 0 {
		return make(map[string]float64), nil
	}

	// CoinGecko API URL for token prices by contract addresses
	// /simple/token_price/{id}?contract_addresses={contract_addresses}&vs_currencies={vs_currencies}
	// Note: {id} here is the asset_platform_id
	apiURL := fmt.Sprintf("%s/simple/token_price/%s", c.cfg.BaseURL, platformID)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	// Устанавливаем URI напрямую в запрос
	req.SetRequestURI(apiURL)

	// Добавляем query параметры к URI запроса
	req.URI().QueryArgs().Add("contract_addresses", strings.Join(contractAddresses, ","))
	req.URI().QueryArgs().Add("vs_currencies", c.cfg.VsCurrency)

	// Add API key if present (for Pro plans, typically in header)
	// For CoinGecko, the free API key is often passed as a query param `x_cg_demo_api_key`
	// or `x_cg_pro_api_key` in header for Pro.
	// The documentation says "x_cg_pro_api_key" for pro plan.
	if c.cfg.APIKey != "" {
		// Check if it's a pro URL, then Pro key should be in header.
		// For simplicity, let's assume if APIKey is set, it's for Pro and goes into header.
		// The official doc says "For Pro API, you are recommended to pass the API key via HTTP header `X-CG-Pro-API-Key`."
		// However, for `/simple/price` the demo key `x_cg_demo_api_key` might be used in query.
		// Let's stick to header for a configured key.
		req.Header.Set("X-CG-Pro-API-Key", c.cfg.APIKey)
	}

	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.SetContentTypeBytes([]byte("application/json"))

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	c.logger.Debug("Requesting token prices from CoinGecko",
		"platform", platformID,
		"url", req.URI().String(),
		"contracts_count", len(contractAddresses),
	)

	// Add timeout to the request if context has a deadline
	// fasthttp doesn't directly use context for per-request timeout like net/http.
	// Client-level ReadTimeout/WriteTimeout apply. For per-request, fasthttp.DoTimeout is used.
	requestTimeout := c.client.ReadTimeout // Use client's read timeout as default
	if deadline, ok := ctx.Deadline(); ok {
		requestTimeout = time.Until(deadline)
		if requestTimeout < 0 { // Deadline already passed
			return nil, fmt.Errorf("context deadline exceeded before making CoinGecko request")
		}
	}

	err := c.client.DoTimeout(req, resp, requestTimeout)
	if err != nil {
		c.logger.Error("CoinGecko API request execution failed", "platform", platformID, "error", err)
		return nil, fmt.Errorf("failed to execute request to CoinGecko for platform %s: %w", platformID, err)
	}

	statusCode := resp.StatusCode()
	respBody := resp.Body()

	c.logger.Debug("Received response from CoinGecko",
		"platform", platformID,
		"status_code", statusCode,
		"body_length", len(respBody),
	)

	if statusCode != fasthttp.StatusOK {
		c.logger.Error("CoinGecko API request failed",
			"platform", platformID,
			"status_code", statusCode,
			"response_body", string(respBody),
		)
		return nil, fmt.Errorf("CoinGecko API request for platform %s failed with status %d: %s", platformID, statusCode, string(respBody))
	}

	// The response is a map where keys are contract addresses (lowercase)
	// and values are maps of currency to price.
	// e.g., {"0xcontractaddress1":{"usd":123.45}, "0xcontractaddress2":{"usd":67.89}}
	var rawPriceData map[string]map[string]float64
	if err := c.json.Unmarshal(respBody, &rawPriceData); err != nil {
		c.logger.Error("Failed to unmarshal CoinGecko response", "platform", platformID, "error", err, "response_body", string(respBody))
		return nil, fmt.Errorf("failed to unmarshal CoinGecko response for platform %s: %w", platformID, err)
	}

	// Transform to map[string]float64 for the configured vsCurrency
	prices := make(map[string]float64)
	for contractAddr, currencyMap := range rawPriceData {
		// Ensure contract address is lowercase for consistent map keys
		lowerContractAddr := strings.ToLower(contractAddr)
		if price, ok := currencyMap[strings.ToLower(c.cfg.VsCurrency)]; ok {
			prices[lowerContractAddr] = price
		} else {
			// This case should ideally not happen if vsCurrency is correctly requested and supported
			c.logger.Warn("Configured vsCurrency not found in CoinGecko response for token",
				"platform", platformID,
				"contract", contractAddr,
				"configured_vs_currency", c.cfg.VsCurrency,
				"available_currencies", currencyMap, // Log available currencies for debugging
			)
		}
	}

	c.logger.Info("Successfully fetched and parsed token prices from CoinGecko", "platform", platformID, "prices_count", len(prices))
	return prices, nil
}

// tempDEXScreenerClientImpl implements DEXScreenerClient - ВРЕМЕННАЯ ЗАГЛУШКА
type tempDEXScreenerClientImpl struct {
	logger Logger // Используем тот же интерфейс логгера
}

// NewDEXScreenerClient creates a new temporary DEXScreener client.
// TODO: Это временная заглушка. Заменить на реальную инициализацию.
func NewDEXScreenerClient(logger Logger) (DEXScreenerClient, error) {
	return &tempDEXScreenerClientImpl{logger: logger}, nil
}

// GetTokenPairsByAddresses is a temporary stub implementation.
// TODO: Это временная заглушка. Реализовать получение данных от DEX Screener.
func (c *tempDEXScreenerClientImpl) GetTokenPairsByAddresses(ctx context.Context, dexscreenerChainID string, tokenAddresses []string) ([]entity.PairData, error) {
	c.logger.Warn("[tempDEXScreenerClientImpl] GetTokenPairsByAddresses called, but not implemented. Returning empty data.", "dexscreenerChainID", dexscreenerChainID, "tokens_count", len(tokenAddresses))
	return []entity.PairData{}, nil // Возвращаем пустой срез и nil ошибку для заглушки
}

// Helper function to check if a string is a valid Ethereum address (basic check)
// func isValidEthereumAddress(address string) bool {
// 	if !strings.HasPrefix(address, "0x") || len(address) != 42 {
// 		return false
// 	}
// 	hexPart := address[2:]
// 	match, _ := regexp.MatchString("^[0-9a-fA-F]+$", hexPart)
// 	return match
// }
