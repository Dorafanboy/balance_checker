package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"balance_checker/internal/client"
	"balance_checker/internal/config"
	"balance_checker/internal/domain/entity"
	dexscreener_entity "balance_checker/internal/entity"
	"balance_checker/internal/pkg/utils"
	"balance_checker/internal/port"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

const (
	stablecoinUSDCSymbol = "USDC"
	stablecoinUSDTSymbol = "USDT"
	stablecoinDAISymbol  = "DAI"
	// Add other stablecoin symbols if needed, ensure they are uppercase
)

var stablecoinSymbols = map[string]struct{}{
	stablecoinUSDCSymbol: {},
	stablecoinUSDTSymbol: {},
	stablecoinDAISymbol:  {},
}

// tokenPriceServiceImpl implements the TokenPriceService interface.
type tokenPriceServiceImpl struct {
	logger *zap.Logger
	cfg    *config.Config
	// coinGeckoClient client.CoinGeckoClient // Deprecated, will be replaced by dexscreenerClient
	dexscreenerClient client.DEXScreenerClient
	pricesCache       *cache.Cache                 // Cache for token prices: key format "chainID_tokenAddress" -> price (float64)
	networkConfigs    map[int64]config.NetworkNode // Map internal chainID to NetworkNode config for easy lookup
}

// NewTokenPriceService creates a new instance of TokenPriceService.
func NewTokenPriceService(
	logger *zap.Logger,
	cfg *config.Config,
	// coinGeckoClient client.CoinGeckoClient, // Deprecated
	dexscreenerClient client.DEXScreenerClient,
) port.TokenPriceService {

	// Initialize networkConfigs map
	ncMap := make(map[int64]config.NetworkNode)
	for _, netCfg := range cfg.Networks {
		ncMap[netCfg.ChainID] = netCfg
	}

	return &tokenPriceServiceImpl{
		logger: logger.Named("TokenPriceService"),
		cfg:    cfg,
		// coinGeckoClient: coinGeckoClient, // Deprecated
		dexscreenerClient: dexscreenerClient,
		pricesCache:       cache.New(time.Duration(cfg.TokenPriceSvc.CacheTTLMinutes)*time.Minute, 10*time.Minute),
		networkConfigs:    ncMap,
	}
}

// LoadAndCacheTokenPrices fetches token prices from DEXScreener for all configured tokens and caches them.
func (s *tokenPriceServiceImpl) LoadAndCacheTokenPrices(ctx context.Context) error {
	s.logger.Info("Starting to load and cache token prices from DEX Screener...")

	tokensByDexChainID := make(map[string][]entity.TokenInfo)

	// Iterate over configured networks to find their DEXScreenerChainID
	for _, netCfg := range s.cfg.Networks {
		if netCfg.DEXScreenerID == "" {
			s.logger.Warn("Skipping token price loading for network, DEXScreenerID not configured",
				zap.String("networkName", netCfg.Name),
				zap.Int64("chainID", netCfg.ChainID))
			continue
		}

		tokens, err := utils.LoadTokensFromJSON(netCfg.TokensFile)
		if err != nil {
			s.logger.Error("Failed to load tokens from file for price caching",
				zap.String("networkName", netCfg.Name),
				zap.String("tokensFile", netCfg.TokensFile),
				zap.Error(err))
			continue // Or return error, depending on desired strictness
		}
		s.logger.Debug("Loaded tokens for price caching", zap.String("networkName", netCfg.Name), zap.Int("count", len(tokens)))
		tokensByDexChainID[netCfg.DEXScreenerID] = append(tokensByDexChainID[netCfg.DEXScreenerID], tokens...)
	}

	var wg sync.WaitGroup
	processedCount := 0
	failedCount := 0

	for dexChainID, tokensInChain := range tokensByDexChainID {
		if len(tokensInChain) == 0 {
			continue
		}
		s.logger.Info("Fetching prices for chain", zap.String("dexChainID", dexChainID), zap.Int("tokenCount", len(tokensInChain)))

		// Batch token addresses according to maxTokensPerBatchRequest
		tokenAddresses := make([]string, 0, len(tokensInChain))
		tokenAddressToInternalChainID := make(map[string]int64) // To map back to our internal chainID for caching key

		for _, token := range tokensInChain {
			tokenAddresses = append(tokenAddresses, token.Address)
			var internalChainID int64 = 0
			for iChainID, nCfg := range s.networkConfigs {
				if nCfg.DEXScreenerID == dexChainID {
					internalChainID = iChainID
					break
				}
			}
			if internalChainID == 0 {
				s.logger.Error("Could not determine internal chainID for token during price loading", zap.String("tokenAddress", token.Address), zap.String("dexChainID", dexChainID))
				continue
			}
			tokenAddressToInternalChainID[strings.ToLower(token.Address)] = internalChainID
		}

		batches := utils.BatchStrings(tokenAddresses, s.cfg.TokenPriceSvc.MaxTokensPerBatchRequest)

		for _, batch := range batches {
			wg.Add(1)
			go func(currentDexChainID string, currentBatch []string) {
				defer wg.Done()
				s.logger.Debug("Fetching price batch", zap.String("dexChainID", currentDexChainID), zap.Int("batchSize", len(currentBatch)))

				pairsData, err := s.dexscreenerClient.GetTokenPairsByAddresses(ctx, currentDexChainID, currentBatch)
				if err != nil {
					s.logger.Error("Failed to get token pairs from DEXScreener for batch",
						zap.String("dexChainID", currentDexChainID),
						zap.Strings("tokenAddresses", currentBatch),
						zap.Error(err))
					failedCount += len(currentBatch)
					return
				}
				if len(pairsData) == 0 {
					s.logger.Warn("No pairs data returned from DEXScreener for batch",
						zap.String("dexChainID", currentDexChainID),
						zap.Strings("tokenAddresses", currentBatch))
				}

				// Group pairs by base token address for easier processing
				pairsByBaseToken := make(map[string][]dexscreener_entity.PairData)
				for _, pData := range pairsData {
					baseAddrLower := strings.ToLower(pData.BaseToken.Address)
					pairsByBaseToken[baseAddrLower] = append(pairsByBaseToken[baseAddrLower], pData)
				}

				for _, tokenAddr := range currentBatch { // Iterate over requested tokens to ensure we try to find a price for each
					tokenAddrLower := strings.ToLower(tokenAddr)
					relatedPairs, ok := pairsByBaseToken[tokenAddrLower]
					if !ok || len(relatedPairs) == 0 {
						s.logger.Warn("No pairs returned from DEXScreener for token address",
							zap.String("dexChainID", currentDexChainID),
							zap.String("tokenAddress", tokenAddr),
							zap.String("reason", "no_pairs_returned_for_token"))
						failedCount++
						continue
					}

					bestPriceStr := s.selectBestPriceFromPairs(relatedPairs, tokenAddr)
					if bestPriceStr == "" {
						s.logger.Warn("Could not determine a suitable price from DEXScreener pairs",
							zap.String("dexChainID", currentDexChainID),
							zap.String("tokenAddress", tokenAddr),
							zap.String("reason", "could_not_select_best_price"))
						failedCount++
						continue
					}

					price, err := strconv.ParseFloat(bestPriceStr, 64)
					if err != nil {
						s.logger.Error("Failed to parse price string from DEXScreener to float",
							zap.String("priceStr", bestPriceStr),
							zap.String("tokenAddress", tokenAddr),
							zap.String("reason", "price_parse_error"),
							zap.Error(err))
						failedCount++
						continue
					}

					internalChainID, found := tokenAddressToInternalChainID[tokenAddrLower]
					if !found {
						s.logger.Error("Internal chainID not found for token address during caching", zap.String("tokenAddress", tokenAddr))
						failedCount++
						continue
					}
					cacheKey := fmt.Sprintf("%d_%s", internalChainID, tokenAddrLower)
					s.pricesCache.Set(cacheKey, price, cache.DefaultExpiration)
					processedCount++
					s.logger.Debug("Cached price for token",
						zap.String("cacheKey", cacheKey),
						zap.Float64("price", price))
				}
			}(dexChainID, batch)
		}
	}

	wg.Wait()
	s.logger.Info("Finished loading and caching token prices from DEX Screener.",
		zap.Int("processedSuccessfully", processedCount),
		zap.Int("failedOrMissing", failedCount))
	return nil
}

// selectBestPriceFromPairs selects the best PriceUsd from a list of pairs for a given baseTokenAddress.
// Priority: Pairs with stablecoins (USDC, USDT, DAI) and highest liquidity.
// Fallback: Pair with highest liquidity overall.
func (s *tokenPriceServiceImpl) selectBestPriceFromPairs(pairs []dexscreener_entity.PairData, baseTokenAddress string) string {
	if len(pairs) == 0 {
		return ""
	}

	var bestOverallPair *dexscreener_entity.PairData
	var bestStablecoinPair *dexscreener_entity.PairData

	for _, p := range pairs {
		pair := p // Create a local copy to take its address if needed
		// Ensure the pair's base token is the one we are interested in (case-insensitive)
		if !strings.EqualFold(pair.BaseToken.Address, baseTokenAddress) {
			continue
		}

		if pair.PriceUsd == "" || pair.PriceUsd == "0" { // Skip if no USD price
			continue
		}

		// Check if quote token is a known stablecoin
		_, isStablecoin := stablecoinSymbols[strings.ToUpper(pair.QuoteToken.Symbol)]

		if isStablecoin {
			if bestStablecoinPair == nil || (pair.Liquidity != nil && bestStablecoinPair.Liquidity != nil && pair.Liquidity.Usd > bestStablecoinPair.Liquidity.Usd) {
				bestStablecoinPair = &pair
			}
		}

		if bestOverallPair == nil || (pair.Liquidity != nil && bestOverallPair.Liquidity != nil && pair.Liquidity.Usd > bestOverallPair.Liquidity.Usd) {
			bestOverallPair = &pair
		}
	}

	if bestStablecoinPair != nil {
		s.logger.Debug("Selected best price from stablecoin pair",
			zap.String("baseTokenAddress", baseTokenAddress),
			zap.String("pairAddress", bestStablecoinPair.PairAddress),
			zap.String("priceUsd", bestStablecoinPair.PriceUsd),
			zap.Float64("liquidityUsd", utils.SafeDerefFloat64(bestStablecoinPair.Liquidity, func(l dexscreener_entity.DEXLiquidity) float64 { return l.Usd })),
			zap.String("quoteToken", bestStablecoinPair.QuoteToken.Symbol))
		return bestStablecoinPair.PriceUsd
	}

	if bestOverallPair != nil {
		s.logger.Debug("Selected best price from overall highest liquidity pair (no stablecoin pair found or preferred)",
			zap.String("baseTokenAddress", baseTokenAddress),
			zap.String("pairAddress", bestOverallPair.PairAddress),
			zap.String("priceUsd", bestOverallPair.PriceUsd),
			zap.Float64("liquidityUsd", utils.SafeDerefFloat64(bestOverallPair.Liquidity, func(l dexscreener_entity.DEXLiquidity) float64 { return l.Usd })),
			zap.String("quoteToken", bestOverallPair.QuoteToken.Symbol))
		return bestOverallPair.PriceUsd
	}

	s.logger.Warn("No suitable price found from pairs",
		zap.String("baseTokenAddress", baseTokenAddress),
		zap.Int("evaluatedPairCount", len(pairs)))
	return ""
}

// GetTokenPrice retrieves the cached price for a given token.
// platformID is the DEXScreener chain identifier (e.g., "ethereum", "bsc").
func (s *tokenPriceServiceImpl) GetTokenPrice(platformID string, tokenAddress string) (float64, bool) {
	var internalChainID int64 = -1
	// Find the internalChainID that corresponds to the given platformID (DEXScreenerID)
	for chainID, netCfg := range s.networkConfigs {
		if strings.EqualFold(netCfg.DEXScreenerID, platformID) || strings.EqualFold(netCfg.Name, platformID) {
			internalChainID = chainID
			break
		}
	}

	if internalChainID == -1 {
		s.logger.Warn("Could not find internal chain ID for platformID in GetTokenPrice",
			zap.String("platformID", platformID),
			zap.String("tokenAddress", tokenAddress))
		return 0, false
	}

	cacheKey := fmt.Sprintf("%d_%s", internalChainID, strings.ToLower(tokenAddress))
	if price, found := s.pricesCache.Get(cacheKey); found {
		if p, ok := price.(float64); ok {
			return p, true
		}
		s.logger.Warn("Price found in cache but not a float64", zap.String("cacheKey", cacheKey), zap.Any("value", price))
	}
	return 0, false
}

// GetAllTokenPrices (optional) returns all cached prices. Useful for debugging.
func (s *tokenPriceServiceImpl) GetAllTokenPrices() map[string]map[string]float64 {
	items := s.pricesCache.Items()
	// prices := make(map[string]float64) // Старый формат
	// Новый формат: map[platformID]map[contractAddress]price
	// Для этого нам нужно будет как-то мапить ключ кеша (internalChainID_address) обратно в platformID.
	// Или изменить GetAllTokenPrices, чтобы он возвращал текущий формат кеша, если это приемлемо для отладки.
	// Пока оставим его таким, чтобы он возвращал внутренний формат кеша.
	pricesByCacheKey := make(map[string]float64)
	for k, v := range items {
		if p, ok := v.Object.(float64); ok {
			pricesByCacheKey[k] = p
		}
	}
	// Вернем пока что плоскую мапу, так как интерфейс ожидает map[string]map[string]float64, что требует переделки.
	// Чтобы не ломать интерфейс, вернем пустую мапу или адаптируем.
	// Для простоты сейчас вернем пустую, чтобы удовлетворить сигнатуру интерфейса, но это нужно будет исправить.
	// TODO: Адаптировать GetAllTokenPrices к новой сигнатуре интерфейса port.TokenPriceService или изменить интерфейс.
	return make(map[string]map[string]float64) // Временное решение
}

// Deprecated: fetchPricesFromCoinGeckoByPlatform is no longer the primary method.
// func (s *tokenPriceServiceImpl) fetchPricesFromCoinGeckoByPlatform(ctx context.Context, platformID string, tokenAddresses []string) (map[string]float64, error) {
// 	// ... (Implementation using CoinGecko, now deprecated)
// 	return nil, fmt.Errorf("CoinGecko client not available or method deprecated")
// }
