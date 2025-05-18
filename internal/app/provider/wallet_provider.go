package provider

import (
	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/infrastructure/walletloader"
)

type walletProviderImpl struct {
	walletFilePath string
	logger         port.Logger
}

// NewWalletProvider creates a new WalletProvider.
func NewWalletProvider(filePath string, logger port.Logger) port.WalletProvider {
	return &walletProviderImpl{walletFilePath: filePath, logger: logger}
}

// GetWallets loads wallet addresses from the configured file.
func (p *walletProviderImpl) GetWallets() ([]entity.Wallet, error) {
	p.logger.Debug("Loading wallets from file", "path", p.walletFilePath)
	wallets, err := walletloader.LoadWallets(p.walletFilePath)
	if err != nil {
		p.logger.Error("Failed to load wallets", "path", p.walletFilePath, "error", err)
		return nil, err
	}
	p.logger.Info("Wallets loaded successfully", "count", len(wallets), "path", p.walletFilePath)
	return wallets, nil
}
