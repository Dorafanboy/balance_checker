package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"balance_checker/internal/config"
	"balance_checker/internal/entity"
	"balance_checker/internal/port"
	"balance_checker/internal/repository"
	"balance_checker/pkg/blockchain"
	"balance_checker/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// portfolioServiceImpl implements the PortfolioService interface.
type portfolioServiceImpl struct {
	repo             repository.PortfolioRepository
	logger           *zap.Logger
	// Deprecated: evmClientProvider will be part of a more generic BlockchainClientProvider
	evmClientProvider blockchain.EVMClientProvider // Used to get specific client for a chain
	cfg               *config.Config
	tokenPriceSvc     port.TokenPriceService
}

// NewPortfolioService creates a new instance of PortfolioService.
func NewPortfolioService(
	repo repository.PortfolioRepository,
	evmClientProvider blockchain.EVMClientProvider,
	tokenPriceSvc port.TokenPriceService,
	cfg *config.Config,
	logger *zap.Logger,
) port.PortfolioService {
	return &portfolioServiceImpl{
		repo:              repo,
		logger:            logger.Named("PortfolioService"),
		evmClientProvider: evmClientProvider,
		cfg:               cfg,
		tokenPriceSvc:     tokenPriceSvc,
	}
}

// GetPortfolio handles the logic to fetch and aggregate token balances for multiple wallet addresses across multiple networks.
func (s *portfolioServiceImpl) GetPortfolio(ctx context.Context, addresses []string, networks []string) (*entity.AggregatedPortfolioResponse, error) {
	s.logger.Info("Fetching portfolio", zap.Strings("addresses", addresses), zap.Strings("networks", networks))

	selectedNetworks, err := s.getSelectedNetworks(networks)
	if err != nil {
		s.logger.Error("Failed to get selected networks", zap.Error(err))
		return nil, fmt.Errorf("failed to get selected networks: %w", err)
	}

	response := &entity.AggregatedPortfolioResponse{
		Portfolios: make([]entity.WalletPortfolio, 0, len(addresses)),
		Errors:     make([]entity.ErrorDetail, 0),
	}

	var mu sync.Mutex // Mutex to protect concurrent writes to response slices
	var grandTotalValueUSD float64

	// Use an errgroup to manage concurrent requests for each address
	eg, childCtx := errgroup.WithContext(ctx)
	eg.SetLimit(s.cfg.PortfolioService.MaxConcurrentRequests) // Limit concurrency for addresses

	for _, address := range addresses {
		addr := address // Capture range variable for goroutine
		eg.Go(func() error {
			walletPortfolio, err := s.fetchSingleWalletPortfolio(childCtx, addr, selectedNetworks)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				s.logger.Error("Error fetching portfolio for address", zap.String("address", addr), zap.Error(err))
				response.Errors = append(response.Errors, entity.ErrorDetail{Address: addr, Message: err.Error()})
				// Continue to process other addresses even if one fails
				return nil // Report as handled to errgroup
			}
			if walletPortfolio != nil {
				response.Portfolios = append(response.Portfolios, *walletPortfolio)
				grandTotalValueUSD += walletPortfolio.TotalValueUSD
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		// This error is from the errgroup itself, not from individual goroutines if they return nil
		s.logger.Error("Error processing wallet portfolios with errgroup", zap.Error(err))
		// Decide if this should be a top-level error or just logged
	}

	response.GrandTotalValueUSD = grandTotalValueUSD
	s.logger.Info("Portfolio fetching complete", zap.Int("portfolioCount", len(response.Portfolios)), zap.Int("errorCount", len(response.Errors)))
	return response, nil
}

func (s *portfolioServiceImpl) fetchSingleWalletPortfolio(ctx context.Context, address string, networksToProcess []config.NetworkNode) (*entity.WalletPortfolio, error) {
	s.logger.Debug("Fetching portfolio for single address", zap.String("address", address), zap.Int("networkCount", len(networksToProcess)))
	walletPortfolio := &entity.WalletPortfolio{
		Address:           address,
		BalancesByNetwork: make(map[string]entity.NetworkTokens),
		TotalValueUSD:     0,
	}
	var walletTotalValueUSD float64
	var mu sync.Mutex // To protect walletPortfolio fields during concurrent network fetches

	// Use another errgroup for fetching balances across networks for this single address
	networkEg, networkCtx := errgroup.WithContext(ctx)
	// Apply a per-address limit if needed, or rely on the global limit of the parent group
	// For now, let's assume parent group limit is sufficient, or set a reasonable one here.
	// networkEg.SetLimit(s.cfg.PortfolioService.MaxConcurrentRequestsPerAddress) // Example if such config existed

	for _, network := range networksToProcess {
		net := network // Capture range variable
		networkEg.Go(func() error {
			networkTokens, err := s.fetchBalancesForNetwork(networkCtx, net, address)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				s.logger.Error("Error fetching balances for network",
					zap.String("address", address),
					zap.String("networkName", net.Name),
					zap.Error(err))
				// Add to per-network errors if structure supports it, or log and continue
				// For now, we don't have per-network error reporting in WalletPortfolio, only top-level errors.
				// If a network fails, its balances won't be included.
				return nil // Report as handled
			}

			if networkTokens != nil && len(networkTokens.Tokens) > 0 {
				walletPortfolio.BalancesByNetwork[net.Name] = *networkTokens
				walletTotalValueUSD += networkTokens.TotalValueUSD
			}
			return nil
		})
	}

	if err := networkEg.Wait(); err != nil {
		// Error from the network processing errgroup
		s.logger.Error("Error processing network balances for address with errgroup",
			zap.String("address", address),
			zap.Error(err))
		// This might indicate a context cancellation or a returned error from one of the goroutines (if they didn't return nil)
		// Depending on strictness, this could be returned as an error for the single wallet portfolio.
		// For now, we proceed with what was successfully fetched.
	}

	walletPortfolio.TotalValueUSD = walletTotalValueUSD

	// Sort NetworkTokens by network name for consistent output
	// This is not strictly necessary if map order is acceptable, but good for tests/consistency.
	// However, map iteration order is not guaranteed. If consistent order is critical,
	// the response structure might need to change from map to slice for BalancesByNetwork.
	// For now, we will rely on the client to handle map display if order matters.

	return walletPortfolio, nil
}

func (s *portfolioServiceImpl) fetchBalancesForNetwork(ctx context.Context, network config.NetworkNode, address string) (*entity.NetworkTokens, error) {
	s.logger.Debug("Fetching balances for network", zap.String("networkName", network.Name), zap.String("address", address))

	client, err := s.evmClientProvider.GetClient(network.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM client for network %s (ChainID: %d): %w", network.Name, network.ChainID, err)
	}

	tokensToFetch, err := s.loadTokensForNetwork(network.TokensFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens for network %s: %w", network.Name, err)
	}

	if len(tokensToFetch) == 0 {
		s.logger.Debug("No tokens configured for network", zap.String("networkName", network.Name))
		return nil, nil // No error, just no tokens
	}

	// Fetch native balance first
	nativeBalanceDetails, err := s.fetchNativeBalance(ctx, client, address, network)
	if err != nil {
		s.logger.Error("Failed to fetch native balance",
			zap.String("network", network.Name),
			zap.String("address", address),
			zap.Error(err))
		// Decide if we should return error or continue with ERC20s. For now, log and continue.
	}

	tokenDetails := make([]entity.TokenDetail, 0, len(tokensToFetch)+1) // +1 for native if present
	var networkTotalValueUSD float64

	if nativeBalanceDetails != nil {
		tokenDetails = append(tokenDetails, *nativeBalanceDetails)
		networkTotalValueUSD += nativeBalanceDetails.ValueUSD
	}

	// Prepare ERC20 token addresses for batch fetching
	erc20TokenAddresses := make([]string, 0, len(tokensToFetch))
	erc20TokenMap := make(map[string]entity.TokenInfo) // For quick lookup after fetching
	for _, token := range tokensToFetch {
		// Skip native token if it was already handled or represented in tokens.json with a specific address
		// (e.g. WETH for ETH). For simplicity, assume tokens.json lists ERC20s.
		if strings.EqualFold(token.Address, entity.ZeroAddress) { // Or a special symbol for native
			continue
		}
		erc20TokenAddresses = append(erc20TokenAddresses, token.Address)
		erc20TokenMap[strings.ToLower(token.Address)] = token
	}

	// Fetch ERC20 balances in batches
	batches := s.batchTokenAddresses(erc20TokenAddresses)
	var mu sync.Mutex // To protect tokenDetails and networkTotalValueUSD

	// Use an errgroup for concurrent batch calls for ERC20 tokens
	erc20eg, erc20Ctx := errgroup.WithContext(ctx)
	limiter := rate.NewLimiter(rate.Limit(s.cfg.RpcClient.RateLimit), s.cfg.RpcClient.BurstLimit)

	for _, batch := range batches {
		currentBatch := batch // Capture range variable
		erc20eg.Go(func() error {
			if err := limiter.Wait(erc20Ctx); err != nil {
				return err // Context cancelled or timeout during wait
			}
			balances, err := client.GetERC20Balances(erc20Ctx, address, currentBatch)
			if err != nil {
				s.logger.Error("Failed to get ERC20 balances for batch",
					zap.String("network", network.Name),
					zap.Strings("batchAddresses", currentBatch),
					zap.Error(err))
				return fmt.Errorf("failed to get ERC20 balances for batch on network %s: %w", network.Name, err)
			}

			mu.Lock()
			defer mu.Unlock()
			for _, bal := range balances {
				if bal.Balance == "0" || bal.Balance == "" { // Skip zero balances
					continue
				}
				tokenInfo, ok := erc20TokenMap[strings.ToLower(bal.TokenAddress)]
				if !ok {
					s.logger.Warn("Got balance for untracked ERC20 token address", zap.String("address", bal.TokenAddress))
					continue
				}

				priceUSD, found := s.tokenPriceSvc.GetTokenPrice(network.ChainID, bal.TokenAddress)
				valueUSD := 0.0
				if found {
					numBalance, err := utils.ParseBigDecimal(bal.Balance)
					if err != nil {
						s.logger.Error("Failed to parse balance to calculate USD value for ERC20 token",
							zap.String("tokenAddress", bal.TokenAddress),
							zap.String("balance", bal.Balance),
							zap.Error(err))
					} else {
						valueUSD = utils.CalculateValueUSD(numBalance, priceUSD, int(tokenInfo.Decimals))
					}
				} else {
					priceUSD = 0.0 // Ensure price is 0 if not found
				}

				detail := entity.TokenDetail{
					TokenAddress:     bal.TokenAddress,
					TokenSymbol:      tokenInfo.Symbol,
					Decimals:         tokenInfo.Decimals,
					FormattedBalance: utils.FormatBalance(bal.Balance, tokenInfo.Decimals, 6),
					PriceUSD:         priceUSD,
					ValueUSD:         valueUSD,
				}
				tokenDetails = append(tokenDetails, detail)
				networkTotalValueUSD += valueUSD
			}
			return nil
		})
	}

	if err := erc20eg.Wait(); err != nil {
		// This error could be from limiter.Wait or from GetERC20Balances if it returned an error
		s.logger.Error("Error fetching ERC20 balances for network",
			zap.String("networkName", network.Name),
			zap.String("address", address),
			zap.Error(err))
		// Depending on how critical this is, we might return an error here.
		// For now, we proceed with successfully fetched balances for this network.
		// return nil, fmt.Errorf("error during ERC20 balance fetching for network %s: %w", network.Name, err)
	}

	if len(tokenDetails) == 0 {
		return nil, nil // No balances found (neither native nor ERC20)
	}

	// Sort tokens by symbol for consistent output
	sort.Slice(tokenDetails, func(i, j int) bool {
		return tokenDetails[i].TokenSymbol < tokenDetails[j].TokenSymbol
	})

	return &entity.NetworkTokens{
		ChainID:       network.ChainID,
		NetworkName:   network.Name,
		Tokens:        tokenDetails,
		TotalValueUSD: networkTotalValueUSD,
	}, nil
}

func (s *portfolioServiceImpl) fetchNativeBalance(ctx context.Context, client blockchain.EVMClient, address string, network config.NetworkNode) (*entity.TokenDetail, error) {
	nativeTokenInfo := s.findNativeTokenInfo(network.TokensFile, network.ChainID)
	if nativeTokenInfo.Symbol == "" { // If no specific native token info, use defaults
		nativeTokenInfo = entity.TokenInfo{
			Symbol:   network.NativeSymbol,
			Decimals: network.NativeDecimals,
			Address:  entity.ZeroAddress, // Or a special identifier for native token
		}
	}

	balance, err := client.GetNativeBalance(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get native balance for %s on %s: %w", address, network.Name, err)
	}

	priceUSD, found := s.tokenPriceSvc.GetTokenPrice(network.ChainID, nativeTokenInfo.Address) // Use address from TokenInfo (could be WETH)
	valueUSD := 0.0
	if found {
		numBalance, err := utils.ParseBigDecimal(balance)
		if err != nil {
			s.logger.Error("Failed to parse balance to calculate USD value for native token",
				zap.String("tokenAddress", nativeTokenInfo.Address),
				zap.String("balance", balance),
				zap.Error(err))
		} else {
			valueUSD = utils.CalculateValueUSD(numBalance, priceUSD, int(nativeTokenInfo.Decimals))
		}
	} else {
		priceUSD = 0.0 // Ensure price is 0 if not found
	}

	return &entity.TokenDetail{
		TokenAddress:     nativeTokenInfo.Address, // This might be a zero address or wrapped native token address
		TokenSymbol:      nativeTokenInfo.Symbol,
		Decimals:         nativeTokenInfo.Decimals,
		FormattedBalance: utils.FormatBalance(balance, nativeTokenInfo.Decimals, 6),
		PriceUSD:         priceUSD,
		ValueUSD:         valueUSD,
	}, nil
}

// findNativeTokenInfo tries to find a specific entry for the native token (e.g., WETH for ETH) in the tokens file.
// This is a helper and might need to be more robust, e.g., by checking a known native token address (like ZeroAddress for price lookup)
// or having standard native token symbols/names per chainID in config.
func (s *portfolioServiceImpl) findNativeTokenInfo(tokensFilePath string, chainID int64) entity.TokenInfo {
	// Attempt to load tokens and find one representing the native currency
	// (e.g., where address is ZeroAddress or a well-known wrapped native token address used for pricing)
	tokens, err := s.loadTokensForNetwork(tokensFilePath)
	if err == nil {
		for _, t := range tokens {
			// Heuristic: native token might be listed with ZeroAddress or as WETH, WBNB etc.
			// For price lookup via GetTokenPrice, we need an address. For native, this is often ZeroAddress or WNative address.
			if strings.EqualFold(t.Address, entity.ZeroAddress) || (t.IsNativeEquivalent != nil && *t.IsNativeEquivalent) {
				// If a specific address is provided for native (like WETH for ETH price), use it for price lookup.
				// Otherwise, ensure we use a consistent address for native price lookup (e.g., ZeroAddress).
				if t.Address == "" { // Ensure address for price lookup is set
					t.Address = entity.ZeroAddress
				}
				return t
			}
		}
	}

	// Fallback to default native token info based on chainID - this needs to be configured or standardized.
	s.logger.Warn("Native token info not found in JSON, using fallback. Consider adding it to tokens file with ZeroAddress or IsNativeEquivalent=true.", zap.Int64("chainID", chainID))
	switch chainID {
	case 1: // Ethereum
		return entity.TokenInfo{Symbol: "ETH", Name: "Ethereum", Decimals: 18, Address: entity.ZeroAddress}
	case 56: // BSC
		return entity.TokenInfo{Symbol: "BNB", Name: "Binance Coin", Decimals: 18, Address: entity.ZeroAddress}
	case 137: // Polygon
		return entity.TokenInfo{Symbol: "MATIC", Name: "Polygon", Decimals: 18, Address: entity.ZeroAddress}
	// Add other common native tokens
	default:
		return entity.TokenInfo{Symbol: "NATIVE", Name: "Native Token", Decimals: 18, Address: entity.ZeroAddress}
	}
}

func (s *portfolioServiceImpl) batchTokenAddresses(addresses []string) [][]string {
	var batches [][]string
	batchSize := s.cfg.PortfolioService.MaxAddressesPerBatchCall
	if batchSize <= 0 {
		batchSize = 20 // Default batch size if not configured or invalid
	}

	for i := 0; i < len(addresses); i += batchSize {
		end := i + batchSize
		if end > len(addresses) {
			end = len(addresses)
		}
		batches = append(batches, addresses[i:end])
	}
	return batches
}

func (s *portfolioServiceImpl) getSelectedNetworks(networkNames []string) ([]config.NetworkNode, error) {
	if len(networkNames) == 0 || (len(networkNames) == 1 && strings.ToLower(networkNames[0]) == "all") {
		return s.cfg.Networks, nil // Return all configured networks
	}

	var selectedNetworks []config.NetworkNode
	configuredNetworksMap := make(map[string]config.NetworkNode)
	for _, net := range s.cfg.Networks {
		configuredNetworksMap[strings.ToLower(net.Name)] = net
		// Allow selection by chainID as well
		configuredNetworksMap[fmt.Sprintf("%d", net.ChainID)] = net
	}

	for _, nameOrID := range networkNames {
		nameOrIDLower := strings.ToLower(nameOrID)
		if net, ok := configuredNetworksMap[nameOrIDLower]; ok {
			selectedNetworks = append(selectedNetworks, net)
		} else {
			return nil, fmt.Errorf("network '%s' not configured or invalid ID", nameOrID)
		}
	}

	if len(selectedNetworks) == 0 {
		return nil, fmt.Errorf("no valid networks selected or specified network list is empty after filtering")
	}
	return selectedNetworks, nil
}

// loadTokensForNetwork loads token information from the JSON file specified in the network configuration.
func (s *portfolioServiceImpl) loadTokensForNetwork(filePath string) ([]entity.TokenInfo, error) {
	if filePath == "" {
		s.logger.Debug("No token file path specified for a network.")
		return []entity.TokenInfo{}, nil
	}
	return utils.LoadTokensFromJSON(filePath)
}

</rewritten_file> 