package httpclient

import (
	"context"
	
	"balance_checker/internal/entity"
)

// DEXScreenerClient defines the interface for interacting with the DEX Screener API.
type DEXScreenerClient interface {
	GetTokenPairsByAddresses(ctx context.Context, dexscreenerChainID string, tokenAddresses []string) ([]entity.PairData, error)
}
