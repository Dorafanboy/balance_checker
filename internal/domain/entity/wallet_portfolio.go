package entity

// WalletPortfolio represents the aggregated balances and any errors
// encountered for a single wallet across all tracked networks and tokens.
type WalletPortfolio struct {
	WalletAddress     string                   `json:"walletAddress"`     // The address of the wallet
	BalancesByNetwork map[string]NetworkTokens `json:"balancesByNetwork"` // Map of network name to its token balances
	TotalValueUSD     float64                  `json:"totalValueUSD"`     // Total value of all tokens for this wallet
}

// TokenDetail represents the details of a specific token balance, excluding network information.
type TokenDetail struct {
	TokenAddress     string  `json:"tokenAddress"`
	TokenSymbol      string  `json:"tokenSymbol"`
	Decimals         uint8   `json:"decimals"`
	FormattedBalance string  `json:"formattedBalance"`
	PriceUSD         float64 `json:"priceUSD"` // Price per token in USD
	ValueUSD         float64 `json:"valueUSD"` // Total value of this balance in USD
}

// NetworkTokens represents all token balances for a specific network.
type NetworkTokens struct {
	ChainID       string        `json:"chainId"`
	Tokens        []TokenDetail `json:"tokens"`
	TotalValueUSD float64       `json:"totalValueUSD"` // Total value of all tokens in this network
}
