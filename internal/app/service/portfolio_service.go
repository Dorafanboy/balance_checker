package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/pkg/utils"
)

// PortfolioServiceImpl implements port.PortfolioService.

// PortfolioServiceImpl implements port.PortfolioService.
type PortfolioServiceImpl struct {
	walletProvider        port.WalletProvider
	networkProvider       port.NetworkDefinitionProvider
	tokenProvider         port.TokenProvider
	clientProvider        port.BlockchainClientProvider
	logger                port.Logger
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
	l port.Logger,
	maxRoutines int,
) port.PortfolioService {
	if maxRoutines <= 0 {
		maxRoutines = 1 // Ensure at least one routine if configured incorrectly
	}
	return &PortfolioServiceImpl{
		walletProvider:        wp,
		networkProvider:       np,
		tokenProvider:         tp,
		clientProvider:        cp,
		logger:                l,
		maxConcurrentRoutines: maxRoutines,
		failedWallets:         make(map[string]bool),
	}
}

// FetchAllWalletsPortfolio fetches portfolios for all wallets defined by the WalletProvider.
func (s *PortfolioServiceImpl) FetchAllWalletsPortfolio(ctx context.Context, trackedNetworkNames []string) ([]entity.WalletPortfolio, []entity.PortfolioError) {
	s.logger.Debug("Fetching all wallets portfolio", "tracked_networks", trackedNetworkNames)
	wallets, err := s.walletProvider.GetWallets()
	if err != nil {
		s.logger.Error("Failed to get wallets", "error", err)
		return nil, []entity.PortfolioError{{Message: fmt.Sprintf("failed to load wallets: %v", err)}}
	}

	activeNetworkDefinitions := make([]entity.NetworkDefinition, 0)
	if s.networkProvider != nil {
		allNetworkDefs := s.networkProvider.GetAllNetworkDefinitions()
		trackedSet := make(map[string]bool)
		for _, name := range trackedNetworkNames {
			trackedSet[name] = true
		}
		for _, netDef := range allNetworkDefs {
			if trackedSet[netDef.Identifier] || trackedSet[netDef.Name] {
				activeNetworkDefinitions = append(activeNetworkDefinitions, netDef)
			}
		}
	}

	if len(activeNetworkDefinitions) == 0 {
		s.logger.Warn("No active (tracked) networks found to process.")
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
			walletPortfolio := s.fetchSingleWalletPortfolio(ctx, w, activeNetworkDefinitions, tokensByChainID, networkSemaphore)
			results <- walletPortfolio
			s.mu.Lock()
			if len(walletPortfolio.Errors) > 0 {
				s.failedWallets[w.Address] = true
				errorMu.Lock()
				allPortfolioErrors = append(allPortfolioErrors, walletPortfolio.Errors...)
				errorMu.Unlock()
			}
			s.mu.Unlock()
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
func (s *PortfolioServiceImpl) fetchSingleWalletPortfolio(ctx context.Context, wallet entity.Wallet, activeNetworks []entity.NetworkDefinition, tokensByChainID map[string][]entity.TokenInfo, networkSemaphore chan struct{}) entity.WalletPortfolio {
	wp := entity.WalletPortfolio{
		WalletAddress: wallet.Address,
		Balances:      []entity.Balance{},
		Errors:        []entity.PortfolioError{},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

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
				wp.Errors = append(wp.Errors, entity.PortfolioError{
					WalletAddress: wallet.Address,
					NetworkName:   nd.Name,
					ChainID:       strconv.FormatUint(nd.ChainID, 10),
					Message:       "Failed to get client: " + err.Error(),
				})
				mu.Unlock()
				return
			}

			tokensForThisNetwork := tokensByChainID[strconv.FormatUint(nd.ChainID, 10)]
			networkBalances, networkErrors := s.fetchBalancesForNetwork(ctx, wallet, nd, client, tokensForThisNetwork)

			mu.Lock()
			wp.Balances = append(wp.Balances, networkBalances...)
			wp.Errors = append(wp.Errors, networkErrors...)
			mu.Unlock()
		}(netDef)
	}

	wg.Wait()
	wp.ErrorCount = len(wp.Errors)
	return wp
}

// fetchBalancesForNetwork fetches native and token balances for a wallet on a specific network.
func (s *PortfolioServiceImpl) fetchBalancesForNetwork(
	ctx context.Context,
	wallet entity.Wallet,
	netDef entity.NetworkDefinition,
	client port.BlockchainClient,
	tokensForNetwork []entity.TokenInfo,
) (balances []entity.Balance, errors []entity.PortfolioError) {

	s.logger.Debug("Fetching balances", "wallet", wallet.Address, "network", netDef.Name)

	nativeChainIDStr := strconv.FormatUint(netDef.ChainID, 10)
	nativeSymbol := netDef.NativeSymbol
	var nativeDecimals int32 = 18
	if netDef.Decimals > 0 {
		nativeDecimals = netDef.Decimals
	}

	nativeBalanceVal, err := client.GetNativeBalance(ctx, wallet.Address)
	if err != nil {
		s.logger.Warn("Failed to get native balance", "wallet", wallet.Address, "network", netDef.Name, "error", err)
		errors = append(errors, entity.PortfolioError{
			WalletAddress: wallet.Address,
			NetworkName:   netDef.Name,
			ChainID:       nativeChainIDStr,
			TokenSymbol:   nativeSymbol,
			IsNative:      true,
			Message:       err.Error(),
		})
	} else {
		if nativeBalanceVal == nil {
			s.logger.Warn("Native balance is nil, skipping", "wallet", wallet.Address, "network", netDef.Name)
		} else {
			formattedNative, errFormat := utils.FormatBigInt(nativeBalanceVal, uint8(nativeDecimals))
			if errFormat != nil {
				s.logger.Warn("Failed to format native balance", "wallet", wallet.Address, "network", netDef.Name, "error", errFormat)
				errors = append(errors, entity.PortfolioError{
					WalletAddress: wallet.Address,
					NetworkName:   netDef.Name,
					ChainID:       nativeChainIDStr,
					TokenSymbol:   nativeSymbol,
					IsNative:      true,
					Message:       "failed to format balance: " + errFormat.Error(),
				})
			} else {
				balances = append(balances, entity.Balance{
					WalletAddress:    wallet.Address,
					NetworkName:      netDef.Name,
					ChainID:          nativeChainIDStr,
					TokenAddress:     "NATIVE",
					TokenSymbol:      nativeSymbol,
					Decimals:         uint8(nativeDecimals),
					IsNative:         true,
					Amount:           nativeBalanceVal,
					FormattedBalance: formattedNative,
				})
			}
		}
	}

	for _, token := range tokensForNetwork {
		tokenChainIDStr := strconv.FormatUint(token.ChainID, 10)
		if tokenChainIDStr != nativeChainIDStr {
			s.logger.Warn("Token ChainID mismatch during balance fetching, skipping token",
				"token_symbol", token.Symbol, "token_address", token.Address,
				"token_chain_id", tokenChainIDStr,
				"network_chain_id", nativeChainIDStr)
			continue
		}
		select {
		case <-ctx.Done():
			s.logger.Info("Context cancelled, stopping token balance fetch", "wallet", wallet.Address, "network", netDef.Name, "token", token.Symbol)
			errors = append(errors, entity.PortfolioError{
				WalletAddress: wallet.Address,
				NetworkName:   netDef.Name,
				ChainID:       tokenChainIDStr,
				TokenSymbol:   token.Symbol,
				TokenAddress:  token.Address,
				IsNative:      false,
				Message:       "Context cancelled",
			})
			continue
		default:
		}

		tokenBalanceVal, err := client.GetTokenBalance(ctx, token.Address, wallet.Address)
		if err != nil {
			s.logger.Warn("Failed to get token balance", "wallet", wallet.Address, "network", netDef.Name, "token", token.Symbol, "error", err)
			errors = append(errors, entity.PortfolioError{
				WalletAddress: wallet.Address,
				NetworkName:   netDef.Name,
				ChainID:       tokenChainIDStr,
				TokenSymbol:   token.Symbol,
				TokenAddress:  token.Address,
				IsNative:      false,
				Message:       err.Error(),
			})
		} else {
			if tokenBalanceVal == nil {
				s.logger.Warn("Token balance is nil, skipping", "wallet", wallet.Address, "network", netDef.Name, "token", token.Symbol)
			} else {
				formattedToken, errFormat := utils.FormatBigInt(tokenBalanceVal, uint8(token.Decimals))
				if errFormat != nil {
					s.logger.Warn("Failed to format token balance", "wallet", wallet.Address, "network", netDef.Name, "token", token.Symbol, "error", errFormat)
					errors = append(errors, entity.PortfolioError{
						WalletAddress: wallet.Address,
						NetworkName:   netDef.Name,
						ChainID:       tokenChainIDStr,
						TokenSymbol:   token.Symbol,
						TokenAddress:  token.Address,
						IsNative:      false,
						Message:       "failed to format balance: " + errFormat.Error(),
					})
				} else {
					balances = append(balances, entity.Balance{
						WalletAddress:    wallet.Address,
						NetworkName:      netDef.Name,
						ChainID:          tokenChainIDStr,
						TokenAddress:     token.Address,
						TokenSymbol:      token.Symbol,
						Decimals:         uint8(token.Decimals),
						IsNative:         false,
						Amount:           tokenBalanceVal,
						FormattedBalance: formattedToken,
					})
				}
			}
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
