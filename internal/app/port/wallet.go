package port

import "balance_checker/internal/domain/entity"

// WalletProvider defines the interface for fetching wallet addresses.
type WalletProvider interface {
	GetWallets() ([]entity.Wallet, error)
}
