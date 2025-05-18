package entity

// WalletPortfolio represents the aggregated balances and any errors
// encountered for a single wallet across all tracked networks and tokens.
type WalletPortfolio struct {
	WalletAddress string           // The address of the wallet
	Balances      []Balance        // List of successfully fetched balances
	Errors        []PortfolioError // List of errors encountered during fetching
	ErrorCount    int              // Total count of errors for this wallet portfolio
}
