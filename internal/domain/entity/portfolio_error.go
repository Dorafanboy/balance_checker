package entity

// PortfolioError represents an error that occurred while fetching
// a part of the portfolio (e.g., balance for a specific token on a network).
type PortfolioError struct {
	WalletAddress string // The address of the wallet for which the error occurred
	NetworkName   string // Name of the network where the error occurred (renamed from NetworkShortName)
	ChainID       string // ChainID of the network where the error occurred
	TokenSymbol   string // Symbol of the token for which the error occurred (can be native symbol)
	TokenAddress  string `json:"tokenAddress,omitempty" yaml:"tokenAddress,omitempty"` // Address of the token, if applicable
	IsNative      bool   // True if the error is related to the native token
	Message       string // The error message
}
