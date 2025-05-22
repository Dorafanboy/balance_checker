package port

import (
	"context"

	"balance_checker/internal/domain/entity"
)

// PortfolioService defines the interface for fetching wallet portfolio information.
type PortfolioService interface {
	FetchAllWalletsPortfolio(ctx context.Context, trackedNetworkNames []string) ([]entity.WalletPortfolio, []entity.PortfolioError)
	FetchSingleWalletPortfolioByAddress(ctx context.Context, walletAddress string, trackedNetworkNames []string) (*entity.WalletPortfolio, []entity.PortfolioError, error)
	GetFailedWallets() []string
}
