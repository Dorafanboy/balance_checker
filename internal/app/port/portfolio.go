package port

import (
	"balance_checker/internal/domain/entity"
	"context"
)

// PortfolioService defines the interface for fetching wallet portfolio information.
type PortfolioService interface {
	// FetchAllWalletsPortfolio fetches portfolios for all wallets, considering only tracked networks.
	// It returns a slice of portfolios and a slice of any critical errors encountered during the process for specific wallet/network/token combinations.
	FetchAllWalletsPortfolio(ctx context.Context, trackedNetworkNames []string) ([]entity.WalletPortfolio, []entity.PortfolioError)

	// FetchSingleWalletPortfolioByAddress fetches portfolio for a single wallet address.
	// It returns the portfolio, partial errors, and a critical error if the wallet/setup is invalid.
	FetchSingleWalletPortfolioByAddress(ctx context.Context, walletAddress string, trackedNetworkNames []string) (*entity.WalletPortfolio, []entity.PortfolioError, error)

	// GetFailedWallets returns a list of wallet addresses for which processing encountered errors.
	GetFailedWallets() []string
}
