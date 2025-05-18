package entity

// TokenInfo holds the details of a specific token.
// This information is typically loaded from configuration files (e.g., data/tokens/[network].json).
type TokenInfo struct {
	ChainID  uint64 // The chain ID of the network this token belongs to
	Address  string // Token contract address
	Name     string // Full name of the token
	Symbol   string // Token symbol (e.g., USDC, ETH)
	Decimals uint8  // Number of decimals the token uses
}
