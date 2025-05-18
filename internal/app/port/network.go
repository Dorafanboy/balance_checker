package port

import (
	"context"
	"math/big"

	"balance_checker/internal/domain/entity"
)

// BlockchainClient defines the interface for interacting with a blockchain network.
// Implementations will be specific to network types (e.g., EVM, Solana).
type BlockchainClient interface {
	// GetNativeBalance fetches the native currency balance (e.g., ETH, BNB) for a wallet.
	GetNativeBalance(ctx context.Context, walletAddress string) (*big.Int, error)

	// GetTokenBalance fetches the balance of a specific token for a wallet.
	GetTokenBalance(ctx context.Context, tokenAddress string, walletAddress string) (*big.Int, error)

	// Definition returns the network definition associated with this client.
	Definition() entity.NetworkDefinition
}

// NetworkDefinitionProvider defines the interface for providing network definitions.
type NetworkDefinitionProvider interface {
	// GetAllNetworkDefinitions returns all available network definitions as a slice.
	GetAllNetworkDefinitions() []entity.NetworkDefinition

	// GetNetworkDefinitionByName returns a specific network definition by its name (or identifier).
	// Возвращает определение и true, если найдено, иначе false.
	GetNetworkDefinitionByName(nameOrIdentifier string) (entity.NetworkDefinition, bool)
}

// BlockchainClientProvider defines the interface for providing blockchain clients.
type BlockchainClientProvider interface {
	GetClient(networkDefinition entity.NetworkDefinition) (BlockchainClient, error)
	// Можно добавить метод для получения всех клиентов, если это необходимо для PortfolioService
	// GetAllActiveClients() map[string]BlockchainClient
}
