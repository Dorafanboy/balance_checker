package port

import (
	"context"

	"balance_checker/internal/domain/entity"
)

// BlockchainClient defines the interface for interacting with a blockchain network.
// Implementations will be specific to network types (e.g., EVM, Solana).
type BlockchainClient interface {
	// GetBalances fetches multiple balances (native and token) for a wallet in a single batch operation if possible.
	// It takes a slice of BalanceRequestItem and returns a corresponding slice of BalanceResultItem.
	// An error can be returned if the entire batch operation fails at a high level.
	// Individual errors for sub-requests should be populated in the Error field of BalanceResultItem.
	GetBalances(ctx context.Context, requests []entity.BalanceRequestItem) ([]entity.BalanceResultItem, error)

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
