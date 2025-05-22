package entity

// WalletPortfolio represents the aggregated balances and any errors
type WalletPortfolio struct {
	WalletAddress     string                   `json:"walletAddress"`
	BalancesByNetwork map[string]NetworkTokens `json:"balancesByNetwork"`
	TotalValueUSD     float64                  `json:"totalValueUSD"`
}

// TokenDetail represents the details of a specific token balance, excluding network information.
type TokenDetail struct {
	TokenAddress     string  `json:"tokenAddress"`
	TokenSymbol      string  `json:"tokenSymbol"`
	Decimals         uint8   `json:"decimals"`
	FormattedBalance string  `json:"formattedBalance"`
	PriceUSD         float64 `json:"priceUSD"`
	ValueUSD         float64 `json:"valueUSD"`
}

// NetworkTokens represents all token balances for a specific network.
type NetworkTokens struct {
	ChainID       string        `json:"chainId"`
	Tokens        []TokenDetail `json:"tokens"`
	TotalValueUSD float64       `json:"totalValueUSD"`
}
