package port

import (
	"balance_checker/internal/domain/entity"
	"context"
)

// TokenProvider defines the interface for fetching token definitions.
type TokenProvider interface {
	// GetTokensByNetwork returns a map of Network ChainID (as string) to a slice of TokenInfo for active networks.
	GetTokensByNetwork(activeNetworkDefs []entity.NetworkDefinition) (map[string][]entity.TokenInfo, error)
}

// TokenPriceService определяет интерфейс для службы получения цен токенов.
type TokenPriceService interface {
	LoadAndCacheTokenPrices(ctx context.Context) error
	GetPriceUSD(dexScreenerChainID string, tokenAddress string) (float64, bool)
	// GetGlobalNativeTokenPrice возвращает цену для глобально отслеживаемого нативного токена, если она есть в кеше.
	GetGlobalNativeTokenPrice(nativeSymbolLower string) (float64, bool)
	// TrySetGlobalNativeTokenPrice пытается установить цену для глобально отслеживаемого нативного токена.
	TrySetGlobalNativeTokenPrice(nativeSymbolLower string, price float64)
}

// BlockchainClient определяет интерфейс для взаимодействия с блокчейном.
