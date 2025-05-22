package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"balance_checker/internal/entity"

	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	dexScreenerBaseURL = "https://api.dexscreener.com/api/v1"
	// tokenPairsByAddressPath = "/tokens/v1/%s/%s" // Старый путь с ошибкой
	tokenPairsByAddressPath = "/tokens/%s/%s" // ИСПРАВЛЕНО: Удален лишний /v1
)

// DEXScreenerClient defines the interface for interacting with the DEX Screener API.
type DEXScreenerClient interface {
	// GetTokenPairsByAddresses fetches pair data for a list of token addresses on a specific chain.
	// It expects the dexscreenerChainID (e.g., "ethereum", "bsc") and a slice of token contract addresses.
	// Returns a slice of PairData entities, where each PairData corresponds to a trading pair associated with one of the requested tokens.
	// A single requested token might result in multiple PairData items if it's part of multiple pairs.
	GetTokenPairsByAddresses(ctx context.Context, dexscreenerChainID string, tokenAddresses []string) ([]entity.PairData, error)
}

// dexScreenerClientImpl is the implementation of DEXScreenerClient.
type dexScreenerClientImpl struct {
	client              *fasthttp.Client
	baseURL             string
	timeout             time.Duration
	logger              *zap.Logger
	maxTokensPerRequest int // Max tokens per single API call (e.g., 30 for DEXScreener)
}

// NewDEXScreenerClient creates a new instance of dexScreenerClientImpl.
func NewDEXScreenerClient(baseURL string, timeout time.Duration, logger *zap.Logger, maxTokensPerRequest int) DEXScreenerClient {
	return &dexScreenerClientImpl{
		client:              &fasthttp.Client{},
		baseURL:             strings.TrimRight(baseURL, "/"),
		timeout:             timeout,
		logger:              logger.Named("DEXScreenerClient"),
		maxTokensPerRequest: maxTokensPerRequest,
	}
}

// GetTokenPairsByAddresses implements the DEXScreenerClient interface.
func (c *dexScreenerClientImpl) GetTokenPairsByAddresses(ctx context.Context, dexscreenerChainID string, tokenAddresses []string) ([]entity.PairData, error) {
	if len(tokenAddresses) == 0 {
		return nil, fmt.Errorf("tokenAddresses cannot be empty")
	}
	if len(tokenAddresses) > c.maxTokensPerRequest {
		c.logger.Warn("Number of token addresses exceeds maxTokensPerRequest",
			zap.Int("requestedCount", len(tokenAddresses)),
			zap.Int("maxAllowed", c.maxTokensPerRequest))
		return nil, fmt.Errorf("number of token addresses (%d) exceeds max tokens per request (%d)", len(tokenAddresses), c.maxTokensPerRequest)
	}

	addresses := strings.Join(tokenAddresses, ",")
	requestURL := fmt.Sprintf("https://api.dexscreener.com/tokens/v1/%s/%s", dexscreenerChainID, addresses)

	c.logger.Debug("Requesting token pairs from DEX Screener", zap.String("url", requestURL))

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(requestURL)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.SetContentTypeBytes([]byte("application/json"))

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	deadline, ok := ctx.Deadline()
	if ok {
		if err := c.client.DoDeadline(req, resp, deadline); err != nil {
			c.logger.Error("Failed to execute request to DEX Screener", zap.String("url", requestURL), zap.Error(err))
			return nil, fmt.Errorf("failed to execute request to %s: %w", requestURL, err)
		}
	} else {
		if err := c.client.DoTimeout(req, resp, c.timeout); err != nil {
			c.logger.Error("Failed to execute request to DEX Screener (with default timeout)", zap.String("url", requestURL), zap.Error(err))
			return nil, fmt.Errorf("failed to execute request to %s with default timeout: %w", requestURL, err)
		}
	}

	rawBody := resp.Body()

	if resp.StatusCode() != fasthttp.StatusOK {
		c.logger.Error("DEX Screener API request failed",
			zap.String("url", requestURL),
			zap.Int("statusCode", resp.StatusCode()),
			zap.ByteString("responseBody", rawBody),
		)
		return nil, fmt.Errorf("DEX Screener API request to %s failed with status %d: %s", requestURL, resp.StatusCode(), string(rawBody))
	}

	var dextpWrapper entity.DEXTokenPair
	if err := json.Unmarshal(rawBody, &dextpWrapper); err == nil && dextpWrapper.Pairs != nil {
		c.logger.Debug("Successfully unmarshalled DEX Screener response (wrapped object)",
			zap.String("dexscreenerChainID", dexscreenerChainID),
			zap.Int("pairCount", len(dextpWrapper.Pairs)))
		if len(dextpWrapper.Pairs) == 0 {
			c.logger.Warn("DEXScreener returned 200 OK with 0 pairs (wrapped object). Check API response.",
				zap.String("url", requestURL),
				zap.String("dexscreenerChainID", dexscreenerChainID),
				zap.ByteString("responseBody", rawBody))
		}
		return dextpWrapper.Pairs, nil
	}

	var directPairs []entity.PairData
	if err := json.Unmarshal(rawBody, &directPairs); err != nil {
		c.logger.Error("Failed to unmarshal DEX Screener response into []PairData (also failed as wrapped DEXTokenPair).",
			zap.String("url", requestURL),
			zap.String("dexscreenerChainID", dexscreenerChainID),
			zap.ByteString("responseBody", rawBody),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to unmarshal DEX Screener response from %s: %w. Body: %s", requestURL, err, string(rawBody))
	}

	if len(directPairs) == 0 {
		c.logger.Warn("DEXScreener returned 200 OK with an empty array of pairs. Check API response.",
			zap.String("url", requestURL),
			zap.String("dexscreenerChainID", dexscreenerChainID),
			zap.ByteString("responseBody", rawBody))
	}

	c.logger.Debug("Successfully unmarshalled DEX Screener response (direct array)",
		zap.String("dexscreenerChainID", dexscreenerChainID),
		zap.Int("pairCount", len(directPairs)))
	return directPairs, nil
}
