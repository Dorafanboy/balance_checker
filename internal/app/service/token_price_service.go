package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	dex_types "balance_checker/internal/entity"
	"balance_checker/internal/infrastructure/configloader"
	"balance_checker/internal/infrastructure/httpclient"
	"balance_checker/internal/pkg/utils"

	"go.uber.org/zap"
)

const (
	stablecoinUSDCSymbol = "USDC"
	stablecoinUSDTSymbol = "USDT"
	stablecoinDAISymbol  = "DAI"
)

var stablecoinSymbols = map[string]struct{}{
	stablecoinUSDCSymbol: {},
	stablecoinUSDTSymbol: {},
	stablecoinDAISymbol:  {},
}

// tokenPriceServiceImpl implements port.TokenPriceService
type tokenPriceServiceImpl struct {
	tokenProvider     port.TokenProvider
	networkProvider   port.NetworkDefinitionProvider
	dexscreenerClient httpclient.DEXScreenerClient
	logger            port.Logger
	cfg               *configloader.Config
	cachedPrices      map[string]map[string]float64
	mu                sync.RWMutex

	globalNativePrices   map[string]float64
	globalNativePricesMu sync.RWMutex
}

// NewTokenPriceService creates a new instance of tokenPriceServiceImpl.
func NewTokenPriceService(
	tp port.TokenProvider,
	np port.NetworkDefinitionProvider,
	dsc httpclient.DEXScreenerClient,
	l port.Logger,
	config *configloader.Config,
) port.TokenPriceService {
	s := &tokenPriceServiceImpl{
		tokenProvider:      tp,
		networkProvider:    np,
		dexscreenerClient:  dsc,
		logger:             l,
		cfg:                config,
		cachedPrices:       make(map[string]map[string]float64),
		globalNativePrices: make(map[string]float64),
	}
	l.Info("TokenPriceService успешно инициализирован.")
	return s
}

// GetGlobalNativeTokenPrice возвращает цену для глобально отслеживаемого нативного токена, если она есть в кеше.
func (s *tokenPriceServiceImpl) GetGlobalNativeTokenPrice(nativeSymbolLower string) (float64, bool) {
	s.globalNativePricesMu.RLock()
	defer s.globalNativePricesMu.RUnlock()
	price, ok := s.globalNativePrices[nativeSymbolLower]
	return price, ok
}

// TrySetGlobalNativeTokenPrice пытается установить цену для глобально отслеживаемого нативного токена.
func (s *tokenPriceServiceImpl) TrySetGlobalNativeTokenPrice(nativeSymbolLower string, price float64) {
	if price <= 0 {
		s.logger.Debug("Attempted to cache zero or negative global native price, skipping.", "symbol", nativeSymbolLower, "price", price)
		return
	}
	s.globalNativePricesMu.Lock()
	defer s.globalNativePricesMu.Unlock()

	s.globalNativePrices[nativeSymbolLower] = price
	s.logger.Info("Global native token price cached/updated", "symbol", nativeSymbolLower, "price", price)
}

// LoadAndCacheTokenPrices implements port.TokenPriceService.
func (s *tokenPriceServiceImpl) LoadAndCacheTokenPrices(ctx context.Context) error {
	s.logger.Info("Starting to load and cache token prices using DEXScreener...")

	activeNetworksForPriceFetching := s.networkProvider.GetAllNetworkDefinitions()
	if len(activeNetworksForPriceFetching) == 0 {
		s.logger.Warn("No active networks found by NetworkDefinitionProvider. Cannot fetch token prices.")
		return nil
	}

	allTokensByChainID, err := s.tokenProvider.GetTokensByNetwork(activeNetworksForPriceFetching)
	if err != nil {
		s.logger.Error("Failed to get all tokens by network from tokenProvider", "error", err)
		return fmt.Errorf("failed to get tokens for price fetching: %w", err)
	}

	var processedSuccessfully, failedOrMissing int
	platformsWithCachedPrices := make(map[string]struct{})

	concurrencyLimit := 5
	if s.cfg != nil && s.cfg.Performance.MaxConcurrentRoutines > 0 {
		concurrencyLimit = s.cfg.Performance.MaxConcurrentRoutines
	}
	sem := make(chan struct{}, concurrencyLimit)
	var wg sync.WaitGroup

	s.mu.Lock()
	for _, netDef := range activeNetworksForPriceFetching {
		dexID := netDef.DEXScreenerChainID
		if dexID != "" {
			if _, ok := s.cachedPrices[dexID]; !ok {
				s.cachedPrices[dexID] = make(map[string]float64)
			}
		}
	}
	s.mu.Unlock()

	for _, netDef := range activeNetworksForPriceFetching {
		currentDexScreenerID := netDef.DEXScreenerChainID

		if currentDexScreenerID == "" {
			s.logger.Warn("DEXScreenerChainID not defined for network, skipping price fetch for its tokens", "network_name", netDef.Name, "network_identifier", netDef.Identifier)
			continue
		}

		tokensForThisNetwork, ok := allTokensByChainID[strconv.FormatUint(netDef.ChainID, 10)]
		if !ok || len(tokensForThisNetwork) == 0 {
			s.logger.Debug("No tokens to fetch prices for or network not found in token map", "network_name", netDef.Name, "dex_screener_id", currentDexScreenerID)
			continue
		}

		s.logger.Info("Fetching prices for DEXScreener chain", "dexScreenerID", currentDexScreenerID, "tokenCount", len(tokensForThisNetwork))

		batches := s.batchTokenInfos(tokensForThisNetwork, s.cfg.TokenPriceSvc.MaxTokensPerBatchRequest)

		for _, batch := range batches {
			if len(batch) == 0 {
				continue
			}
			wg.Add(1)
			sem <- struct{}{}

			go func(batch []entity.TokenInfo, dexscreenerID string, networkName string, networkIdentifier string) {
				defer wg.Done()
				defer func() { <-sem }()

				tokenAddresses := make([]string, len(batch))
				for i, token := range batch {
					tokenAddresses[i] = token.Address
				}

				pairs, err := s.dexscreenerClient.GetTokenPairsByAddresses(ctx, dexscreenerID, tokenAddresses)
				if err != nil {
					s.logger.Error("Failed to get token pairs from DEXScreener",
						"dexScreenerID", dexscreenerID,
						"token_addresses_count", len(tokenAddresses),
						"error", err)
					failedOrMissing += len(batch)
					return
				}

				foundPricesForBatch := 0
				for _, tokenInfo := range batch {
					foundPair := false
					for _, pair := range pairs {
						if strings.EqualFold(pair.BaseToken.Address, tokenInfo.Address) {
							price, errConv := strconv.ParseFloat(pair.PriceUsd, 64)
							if errConv != nil {
								s.logger.Warn("Failed to parse token price from DEXScreener",
									"dexScreenerID", dexscreenerID,
									"tokenAddress", tokenInfo.Address,
									"price_string", pair.PriceUsd,
									"error", errConv)
								failedOrMissing++
								continue
							}

							s.mu.Lock()
							s.cachedPrices[dexscreenerID][strings.ToLower(tokenInfo.Address)] = price
							s.mu.Unlock()

							s.logger.Debug("Cached price for token",
								"dexScreenerID", dexscreenerID,
								"tokenAddress", tokenInfo.Address,
								"priceUSD", price)
							processedSuccessfully++
							foundPricesForBatch++
							platformsWithCachedPrices[dexscreenerID] = struct{}{}
							foundPair = true
							break
						}
					}
					if !foundPair {
						s.logger.Warn("No pairs returned from DEXScreener for token address",
							"dexScreenerID", dexscreenerID,
							"tokenAddress", tokenInfo.Address,
							"reason", "no_pairs_returned_for_token")
						failedOrMissing++
					}
				}
			}(batch, currentDexScreenerID, netDef.Name, netDef.Identifier)
		}
	}

	wg.Wait()
	s.logger.Info("Finished loading and caching token prices from DEXScreener.",
		zap.Int("processedSuccessfully", processedSuccessfully),
		zap.Int("failedOrMissing", failedOrMissing),
		zap.Int("totalPlatformsWithCachedPrices", len(platformsWithCachedPrices)))
	return nil
}

// selectBestPriceFromPairs (перенесен из старого сервиса и адаптирован)
func (s *tokenPriceServiceImpl) selectBestPriceFromPairs(pairs []dex_types.PairData, baseTokenAddress string) string {
	if len(pairs) == 0 {
		return ""
	}

	var bestOverallPair *dex_types.PairData
	var bestStablecoinPair *dex_types.PairData

	for i := range pairs {
		pair := &pairs[i]
		if !strings.EqualFold(pair.BaseToken.Address, baseTokenAddress) {
			continue
		}
		if pair.PriceUsd == "" || pair.PriceUsd == "0" {
			continue
		}

		_, isStablecoin := stablecoinSymbols[strings.ToUpper(pair.QuoteToken.Symbol)]

		if isStablecoin {
			if bestStablecoinPair == nil || (pair.Liquidity != nil && bestStablecoinPair.Liquidity != nil && pair.Liquidity.Usd > bestStablecoinPair.Liquidity.Usd) {
				bestStablecoinPair = pair
			}
		}
		if bestOverallPair == nil || (pair.Liquidity != nil && bestOverallPair.Liquidity != nil && pair.Liquidity.Usd > bestOverallPair.Liquidity.Usd) {
			bestOverallPair = pair
		}
	}

	if bestStablecoinPair != nil {
		s.logger.Debug("Selected best price from stablecoin pair",
			"baseTokenAddress", baseTokenAddress,
			"pairAddress", bestStablecoinPair.PairAddress,
			"priceUsd", bestStablecoinPair.PriceUsd,
			"liquidityUsd", utils.SafeDerefFloat64(bestStablecoinPair.Liquidity, func(l dex_types.DEXLiquidity) float64 { return l.Usd }),
			"quoteToken", bestStablecoinPair.QuoteToken.Symbol)
		return bestStablecoinPair.PriceUsd
	}

	if bestOverallPair != nil {
		s.logger.Debug("Selected best price from overall highest liquidity pair",
			"baseTokenAddress", baseTokenAddress,
			"pairAddress", bestOverallPair.PairAddress,
			"priceUsd", bestOverallPair.PriceUsd,
			"liquidityUsd", utils.SafeDerefFloat64(bestOverallPair.Liquidity, func(l dex_types.DEXLiquidity) float64 { return l.Usd }),
			"quoteToken", bestOverallPair.QuoteToken.Symbol)
		return bestOverallPair.PriceUsd
	}

	s.logger.Warn("No suitable price found from pairs",
		"baseTokenAddress", baseTokenAddress,
		"evaluatedPairCount", len(pairs))
	return ""
}

// GetPriceUSD реализует port.TokenPriceService и возвращает цену токена из кеша.
func (s *tokenPriceServiceImpl) GetPriceUSD(dexScreenerChainID string, tokenAddress string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if chainPrices, ok := s.cachedPrices[dexScreenerChainID]; ok {
		price, found := chainPrices[strings.ToLower(tokenAddress)]
		return price, found
	}
	return 0, false
}

// GetAllTokenPrices implements port.TokenPriceService.
func (s *tokenPriceServiceImpl) GetAllTokenPrices() map[string]map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pricesCopy := make(map[string]map[string]float64)
	for platform, addrMap := range s.cachedPrices {
		pricesCopy[platform] = make(map[string]float64)
		for addr, price := range addrMap {
			pricesCopy[platform][addr] = price
		}
	}
	return pricesCopy
}

// Вспомогательная функция для разделения среза токенов на батчи
func (s *tokenPriceServiceImpl) batchTokenInfos(tokens []entity.TokenInfo, batchSize int) [][]entity.TokenInfo {
	var batches [][]entity.TokenInfo
	if batchSize <= 0 {
		batchSize = 30
	}

	for i := 0; i < len(tokens); i += batchSize {
		end := i + batchSize
		if end > len(tokens) {
			end = len(tokens)
		}
		batches = append(batches, tokens[i:end])
	}
	return batches
}
