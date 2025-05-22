package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/infrastructure/configloader"
	"balance_checker/internal/pkg/utils"
)

// PortfolioServiceImpl implements port.PortfolioService.
type PortfolioServiceImpl struct {
	walletProvider        port.WalletProvider
	networkProvider       port.NetworkDefinitionProvider
	tokenProvider         port.TokenProvider
	clientProvider        port.BlockchainClientProvider
	tokenPriceSvc         port.TokenPriceService
	logger                port.Logger
	cfg                   *configloader.Config
	maxConcurrentRoutines int
	failedWallets         map[string]bool
	mu                    sync.Mutex
}

// NewPortfolioService creates a new instance of PortfolioServiceImpl.
func NewPortfolioService(
	wp port.WalletProvider,
	np port.NetworkDefinitionProvider,
	tp port.TokenProvider,
	cp port.BlockchainClientProvider,
	tps port.TokenPriceService,
	l port.Logger,
	config *configloader.Config,
	maxRoutines int,
) port.PortfolioService {
	if maxRoutines <= 0 {
		maxRoutines = 1
	}
	return &PortfolioServiceImpl{
		walletProvider:        wp,
		networkProvider:       np,
		tokenProvider:         tp,
		clientProvider:        cp,
		tokenPriceSvc:         tps,
		logger:                l,
		cfg:                   config,
		maxConcurrentRoutines: maxRoutines,
		failedWallets:         make(map[string]bool),
	}
}

// FetchAllWalletsPortfolio fetches portfolios for all wallets defined by the WalletProvider.
func (s *PortfolioServiceImpl) FetchAllWalletsPortfolio(
	ctx context.Context,
	trackedNetworkNames []string,
) ([]entity.WalletPortfolio, []entity.PortfolioError) {
	s.logger.Debug("Fetching all wallets portfolio", "tracked_networks", trackedNetworkNames)
	wallets, err := s.walletProvider.GetWallets()
	if err != nil {
		s.logger.Error("Failed to get wallets", "error", err)
		return nil, []entity.PortfolioError{{Message: fmt.Sprintf("failed to load wallets: %v", err)}}
	}

	var activeNetworkDefinitions []entity.NetworkDefinition
	if s.networkProvider != nil {
		allNetworkDefs := s.networkProvider.GetAllNetworkDefinitions()

		if len(trackedNetworkNames) == 0 {
			activeNetworkDefinitions = allNetworkDefs
			s.logger.Debug("No specific tracked networks provided, using all networks from NetworkDefinitionProvider.",
				"count", len(activeNetworkDefinitions))
		} else {
			filteredDefinitions := make([]entity.NetworkDefinition, 0)
			trackedSet := make(map[string]bool)
			for _, name := range trackedNetworkNames {
				trackedSet[strings.ToLower(name)] = true
			}
			for _, netDef := range allNetworkDefs {
				if trackedSet[strings.ToLower(netDef.Identifier)] {
					filteredDefinitions = append(filteredDefinitions, netDef)
				}
			}
			activeNetworkDefinitions = filteredDefinitions
			s.logger.Debug("Filtered networks based on trackedNetworkNames.", "initial_count",
				len(allNetworkDefs), "tracked_count", len(trackedNetworkNames), "final_count", len(activeNetworkDefinitions))
		}
	}

	if len(activeNetworkDefinitions) == 0 {
		s.logger.Warn("No active networks found to process (either no networks defined by provider or filter mismatch).")
		return []entity.WalletPortfolio{}, nil
	}
	s.logger.Debug("Active networks to process", "count", len(activeNetworkDefinitions))

	tokensByChainID, err := s.tokenProvider.GetTokensByNetwork(activeNetworkDefinitions)
	if err != nil {
		s.logger.Error("Failed to get tokens by network", "error", err)
		return nil, []entity.PortfolioError{{Message: fmt.Sprintf("failed to load tokens: %v", err)}}
	}

	results := make(chan entity.WalletPortfolio, len(wallets))
	var wg sync.WaitGroup
	networkSemaphore := make(chan struct{}, s.maxConcurrentRoutines)

	var allPortfolioErrors []entity.PortfolioError
	var errorMu sync.Mutex

	for _, wallet := range wallets {
		wg.Add(1)
		go func(w entity.Wallet) {
			defer wg.Done()
			s.logger.Debug("Fetching portfolio for wallet", "address", w.Address)
			walletPortfolioResult, singleWalletErrors := s.fetchSingleWalletPortfolio(ctx, w, activeNetworkDefinitions, tokensByChainID, networkSemaphore)
			results <- walletPortfolioResult

			if len(singleWalletErrors) > 0 {
				s.mu.Lock()
				s.failedWallets[w.Address] = true
				s.mu.Unlock()

				errorMu.Lock()
				allPortfolioErrors = append(allPortfolioErrors, singleWalletErrors...)
				errorMu.Unlock()
			}
		}(wallet)
	}

	wg.Wait()
	close(results)

	allWalletsPortfolios := make([]entity.WalletPortfolio, 0, len(wallets))
	for p := range results {
		allWalletsPortfolios = append(allWalletsPortfolios, p)
	}

	s.logger.Info("Successfully fetched portfolios for all wallets", "count", len(allWalletsPortfolios))
	return allWalletsPortfolios, allPortfolioErrors
}

// fetchSingleWalletPortfolio fetches the portfolio for a single wallet across specified active networks.
func (s *PortfolioServiceImpl) fetchSingleWalletPortfolio(
	ctx context.Context,
	wallet entity.Wallet,
	activeNetworks []entity.NetworkDefinition,
	tokensByChainID map[string][]entity.TokenInfo,
	networkSemaphore chan struct{},
) (entity.WalletPortfolio, []entity.PortfolioError) {
	s.logger.Debug("Fetching portfolio for single wallet", "wallet_address", wallet.Address, "active_networks_count", len(activeNetworks))

	globallyPricedNativeSymbols := map[string]struct{}{
		"eth": {},
	}

	portfolio := entity.WalletPortfolio{
		WalletAddress:     wallet.Address,
		BalancesByNetwork: make(map[string]entity.NetworkTokens),
	}
	var singleWalletErrs []entity.PortfolioError

	var mu sync.Mutex
	var wg sync.WaitGroup
	var overallWalletTotalValueUSD float64 = 0

	for _, netDef := range activeNetworks {
		networkSemaphore <- struct{}{}
		wg.Add(1)
		go func(nd entity.NetworkDefinition) {
			defer wg.Done()
			defer func() { <-networkSemaphore }()

			client, err := s.clientProvider.GetClient(nd)
			if err != nil {
				s.logger.Error("Failed to get blockchain client for network", "network", nd.Name, "error", err)
				mu.Lock()
				singleWalletErrs = append(singleWalletErrs, entity.PortfolioError{
					WalletAddress: wallet.Address, NetworkName: nd.Name, ChainID: strconv.FormatUint(nd.ChainID, 10), Message: "Failed to get client: " + err.Error()})
				mu.Unlock()
				return
			}

			tokensForThisNetwork := tokensByChainID[strconv.FormatUint(nd.ChainID, 10)]
			networkBalances, networkErrors := s.fetchBalancesForNetwork(ctx, wallet, nd, client, tokensForThisNetwork)

			mu.Lock()
			if len(networkErrors) > 0 {
				singleWalletErrs = append(singleWalletErrs, networkErrors...)
			}

			var networkTotalValueUSD float64 = 0
			currentTokenDetails := make([]entity.TokenDetail, 0, len(networkBalances))

			if len(networkBalances) > 0 {
				currentDexscreenerID := nd.DEXScreenerChainID
				canFetchPrices := currentDexscreenerID != ""

				if !canFetchPrices {
					s.logger.Warn("DEXScreenerChainID not defined for network in its NetworkDefinition. Prices will be zero for all tokens in this network.",
						"networkIdentifier", nd.Identifier,
						"networkName", nd.Name)
				}

				for _, nb := range networkBalances {
					td := entity.TokenDetail{
						TokenAddress:     nb.TokenAddress,
						TokenSymbol:      nb.TokenSymbol,
						Decimals:         nb.Decimals,
						FormattedBalance: nb.FormattedBalance,
					}

					var priceUSD float64
					var valueUSD float64

					isNative := nb.TokenAddress == "" || strings.EqualFold(nb.TokenAddress, entity.ZeroAddress)

					if isNative {
						nativeSymbolLower := strings.ToLower(nd.NativeSymbol)
						_, isGloballyPricedAsset := globallyPricedNativeSymbols[nativeSymbolLower]

						if isGloballyPricedAsset {
							cachedGlobalPrice, foundGlobal := s.tokenPriceSvc.GetGlobalNativeTokenPrice(nativeSymbolLower)
							if foundGlobal {
								priceUSD = cachedGlobalPrice
								s.logger.Debug("Using globally cached price for native asset", "symbol", nd.NativeSymbol, "price", priceUSD, "network", nd.Name)
							}
						}

						if priceUSD == 0 {
							if canFetchPrices && nd.WrappedNativeTokenAddress != "" && !strings.EqualFold(nd.WrappedNativeTokenAddress, entity.ZeroAddress) {
								wrappedAddressLower := strings.ToLower(nd.WrappedNativeTokenAddress)
								fetchedPrice, priceFound := s.tokenPriceSvc.GetPriceUSD(currentDexscreenerID, wrappedAddressLower)

								if priceFound && fetchedPrice > 0 {
									priceUSD = fetchedPrice
									s.logger.Debug("Fetched price for NATIVE asset via wrapped address",
										"nativeSymbol", nd.NativeSymbol, "wrappedAddress", wrappedAddressLower,
										"price", priceUSD, "network", nd.Name, "dexscreenerID", currentDexscreenerID)

									if isGloballyPricedAsset {
										s.tokenPriceSvc.TrySetGlobalNativeTokenPrice(nativeSymbolLower, priceUSD)
									}
								} else if !priceFound {
									s.logger.Warn("Price not found or zero for NATIVE asset (via wrapped)",
										"tokenSymbol", nd.NativeSymbol, "wrappedAddressAttempted", wrappedAddressLower,
										"network", nd.Name, "dexscreenerID", currentDexscreenerID)
								}
							} else if nd.WrappedNativeTokenAddress == "" || strings.EqualFold(nd.WrappedNativeTokenAddress, entity.ZeroAddress) {
								s.logger.Warn("WrappedNativeTokenAddress is not defined or is zero address for NATIVE asset, cannot fetch price.",
									"tokenSymbol", nd.NativeSymbol, "network", nd.Name)
							}
						}
					} else {
						if canFetchPrices {
							tokenAddressLower := strings.ToLower(nb.TokenAddress)
							fetchedPrice, priceFound := s.tokenPriceSvc.GetPriceUSD(currentDexscreenerID, tokenAddressLower)
							if priceFound && fetchedPrice > 0 {
								priceUSD = fetchedPrice
							} else if !priceFound {
								s.logger.Warn("Price not found or zero for ERC20 TOKEN",
									"tokenSymbol", nb.TokenSymbol, "tokenAddress", tokenAddressLower,
									"network", nd.Name, "dexscreenerID", currentDexscreenerID)
							}
						}
					}

					if nb.Amount != nil && priceUSD > 0 {
						calculatedValue, errCalc := utils.CalculateValueUSD(nb.Amount, nb.Decimals, priceUSD)
						if errCalc != nil {
							s.logger.Error("Failed to calculate valueUSD",
								"wallet", wallet.Address,
								"network", nd.Name,
								"token", nb.TokenSymbol,
								"raw_amount", nb.Amount.String(),
								"price", priceUSD,
								"error", errCalc)
						} else {
							valueUSD = calculatedValue
						}
					}

					td.PriceUSD = priceUSD
					td.ValueUSD = valueUSD
					currentTokenDetails = append(currentTokenDetails, td)
					networkTotalValueUSD += valueUSD
				}

				if len(currentTokenDetails) > 0 {
					networkChainIDStr := strconv.FormatUint(nd.ChainID, 10)
					networkTokenEntry := entity.NetworkTokens{
						ChainID: networkChainIDStr,
						Tokens:  currentTokenDetails,
					}
					networkTokenEntry.TotalValueUSD = networkTotalValueUSD
					overallWalletTotalValueUSD += networkTotalValueUSD
					portfolio.BalancesByNetwork[nd.Name] = networkTokenEntry
				}
			}
			mu.Unlock()
		}(netDef)
	}

	wg.Wait()

	portfolio.TotalValueUSD = overallWalletTotalValueUSD

	return portfolio, singleWalletErrs
}

// fetchBalancesForNetwork fetches native and token balances for a wallet on a specific network.
func (s *PortfolioServiceImpl) fetchBalancesForNetwork(
	ctx context.Context,
	wallet entity.Wallet,
	netDef entity.NetworkDefinition,
	client port.BlockchainClient,
	tokensForNetwork []entity.TokenInfo,
) (balances []entity.Balance, errors []entity.PortfolioError) {
	s.logger.Debug("Preparing batch balance request (prices are cached)", "wallet",
		wallet.Address, "network", netDef.Name, "token_count", len(tokensForNetwork))

	var balanceRequests []entity.BalanceRequestItem
	nativeDecimals := netDef.Decimals
	if nativeDecimals == 0 {
		nativeDecimals = 18
	}
	balanceRequests = append(balanceRequests, entity.BalanceRequestItem{
		ID:            fmt.Sprintf("%s-%s-NATIVE", wallet.Address, netDef.Identifier),
		Type:          entity.NativeBalanceRequest,
		WalletAddress: wallet.Address,
		TokenSymbol:   netDef.NativeSymbol,
		TokenDecimals: uint8(nativeDecimals),
	})

	for _, token := range tokensForNetwork {
		if strconv.FormatUint(token.ChainID, 10) != strconv.FormatUint(netDef.ChainID, 10) {
			s.logger.Warn("Token ChainID mismatch, skipping token in batch preparation",
				"wallet", wallet.Address, "network", netDef.Name, "token_symbol", token.Symbol,
				"token_chain_id", token.ChainID, "network_chain_id", netDef.ChainID)
			continue
		}
		balanceRequests = append(balanceRequests, entity.BalanceRequestItem{
			ID:            fmt.Sprintf("%s-%s-%s", wallet.Address, netDef.Identifier, token.Address),
			Type:          entity.TokenBalanceRequest,
			WalletAddress: wallet.Address,
			TokenAddress:  token.Address,
			TokenSymbol:   token.Symbol,
			TokenDecimals: token.Decimals,
		})
	}

	if len(balanceRequests) == 0 {
		s.logger.Debug("No balance requests to send for wallet (prices cached)", "wallet", wallet.Address, "network", netDef.Name)
		return nil, nil
	}

	s.logger.Debug("Executing batch balance request (prices cached)", "wallet", wallet.Address, "network", netDef.Name, "request_count", len(balanceRequests))
	batchResults, err := client.GetBalances(ctx, balanceRequests)
	if err != nil {
		s.logger.Error("Batch GetBalances call failed for network (prices cached)", "wallet", wallet.Address, "network", netDef.Name, "error", err)
		errors = append(errors, entity.PortfolioError{
			WalletAddress: wallet.Address, NetworkName: netDef.Name, ChainID: strconv.FormatUint(netDef.ChainID, 10), Message: fmt.Sprintf("batch balance fetch failed: %v", err)})
		return nil, errors
	}

	s.logger.Debug("Processing batch balance results (prices cached)", "wallet", wallet.Address, "network", netDef.Name, "result_count", len(batchResults))
	for _, resItem := range batchResults {
		if resItem.Error != nil {
			s.logger.Warn("Error in batch balance sub-request (prices cached)",
				"wallet", resItem.WalletAddress, "network", netDef.Name,
				"token_symbol", resItem.TokenSymbol, "token_address", resItem.TokenAddress, "error", resItem.Error)
			errors = append(errors, entity.PortfolioError{
				WalletAddress: resItem.WalletAddress, NetworkName: netDef.Name, ChainID: strconv.FormatUint(netDef.ChainID, 10),
				TokenSymbol: resItem.TokenSymbol, TokenAddress: resItem.TokenAddress, IsNative: resItem.IsNative, Message: resItem.Error.Error()})
			continue
		}

		if resItem.Balance != nil && resItem.Balance.Sign() != 0 {
			balances = append(balances, entity.Balance{
				WalletAddress:    resItem.WalletAddress,
				NetworkName:      netDef.Name,
				ChainID:          strconv.FormatUint(netDef.ChainID, 10),
				TokenAddress:     resItem.TokenAddress,
				TokenSymbol:      resItem.TokenSymbol,
				Decimals:         resItem.Decimals,
				IsNative:         resItem.IsNative,
				Amount:           resItem.Balance,
				FormattedBalance: resItem.FormattedBalance,
			})
		} else {
			s.logger.Debug("Skipping zero or nil balance from batch result (prices cached)",
				"wallet", resItem.WalletAddress, "network", netDef.Name, "token", resItem.TokenSymbol)
		}
	}

	return balances, errors
}

// GetFailedWallets returns a list of wallet addresses for which fetching failed.
func (s *PortfolioServiceImpl) GetFailedWallets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	failed := make([]string, 0, len(s.failedWallets))
	for addr, failedStatus := range s.failedWallets {
		if failedStatus {
			failed = append(failed, addr)
		}
	}
	return failed
}

// FetchSingleWalletPortfolioByAddress fetches portfolio for a single wallet address.
func (s *PortfolioServiceImpl) FetchSingleWalletPortfolioByAddress(ctx context.Context, walletAddress string, trackedNetworkNames []string) (*entity.WalletPortfolio, []entity.PortfolioError, error) {
	s.logger.Debug("Fetching single wallet portfolio by address", "wallet_address", walletAddress, "tracked_networks", trackedNetworkNames)

	wallet, err := s.walletProvider.GetWalletByAddress(walletAddress)
	if err != nil {
		s.logger.Warn("Wallet not found by address", "address", walletAddress, "error", err)
		return nil, nil, fmt.Errorf("wallet with address %s not found", walletAddress)
	}

	var activeNetworkDefinitions []entity.NetworkDefinition
	if s.networkProvider != nil {
		allNetworkDefs := s.networkProvider.GetAllNetworkDefinitions()
		if len(trackedNetworkNames) == 0 {
			activeNetworkDefinitions = allNetworkDefs
			s.logger.Debug("Using all active networks from provider for single wallet", "count", len(activeNetworkDefinitions))
		} else {
			filteredDefinitions := make([]entity.NetworkDefinition, 0)
			trackedSet := make(map[string]bool)
			for _, name := range trackedNetworkNames {
				trackedSet[strings.ToLower(name)] = true
			}
			for _, netDef := range allNetworkDefs {
				if trackedSet[strings.ToLower(netDef.Identifier)] {
					filteredDefinitions = append(filteredDefinitions, netDef)
				}
			}
			activeNetworkDefinitions = filteredDefinitions
			s.logger.Debug("Filtered networks for single wallet", "initial_count", len(allNetworkDefs), "tracked_count", len(trackedNetworkNames), "final_count", len(activeNetworkDefinitions))
		}
	}

	if len(activeNetworkDefinitions) == 0 {
		s.logger.Warn("No active or tracked networks found for processing for single wallet", "wallet_address", walletAddress)
		return nil, nil, fmt.Errorf("no active or tracked networks found for wallet %s", walletAddress)
	}
	s.logger.Debug("Active networks to process for single wallet", "count", len(activeNetworkDefinitions))

	tokensByChainID, err := s.tokenProvider.GetTokensByNetwork(activeNetworkDefinitions)
	if err != nil {
		s.logger.Error("Failed to get tokens by network for single wallet", "wallet_address", walletAddress, "error", err)
		return nil, nil, fmt.Errorf("failed to load tokens for wallet %s: %w", walletAddress, err)
	}

	networkSemaphore := make(chan struct{}, s.maxConcurrentRoutines)

	walletPortfolioResult, singleWalletErrors := s.fetchSingleWalletPortfolio(ctx, *wallet, activeNetworkDefinitions, tokensByChainID, networkSemaphore)

	if len(singleWalletErrors) > 0 {
		s.mu.Lock()
		s.failedWallets[wallet.Address] = true
		s.mu.Unlock()
	}

	s.logger.Info("Successfully fetched portfolio for single wallet", "address", wallet.Address)
	return &walletPortfolioResult, singleWalletErrors, nil
}
