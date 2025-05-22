package port

import (
	"context"

	"balance_checker/internal/domain/entity"
)

// TokenProvider defines the interface for fetching token definitions.
type TokenProvider interface {
	GetTokensByNetwork(activeNetworkDefs []entity.NetworkDefinition) (map[string][]entity.TokenInfo, error)
}

// TokenPriceService определяет интерфейс для службы получения цен токенов.
type TokenPriceService interface {
	LoadAndCacheTokenPrices(ctx context.Context) error
	GetPriceUSD(dexScreenerChainID string, tokenAddress string) (float64, bool)
	GetGlobalNativeTokenPrice(nativeSymbolLower string) (float64, bool)
	TrySetGlobalNativeTokenPrice(nativeSymbolLower string, price float64)
}
