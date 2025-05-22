package entity

// PortfolioError represents an error that occurred while fetching
type PortfolioError struct {
	WalletAddress string
	NetworkName   string
	ChainID       string
	TokenSymbol   string
	TokenAddress  string `json:"tokenAddress,omitempty" yaml:"tokenAddress,omitempty"`
	IsNative      bool
	Message       string
}
