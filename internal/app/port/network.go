package port

import (
	"context"

	"balance_checker/internal/domain/entity"
)

// BlockchainClient defines the interface for interacting with a blockchain network.
type BlockchainClient interface {
	GetBalances(ctx context.Context, requests []entity.BalanceRequestItem) ([]entity.BalanceResultItem, error)
	Definition() entity.NetworkDefinition
}

// NetworkDefinitionProvider defines the interface for providing network definitions.
type NetworkDefinitionProvider interface {
	GetAllNetworkDefinitions() []entity.NetworkDefinition
	GetNetworkDefinitionByName(nameOrIdentifier string) (entity.NetworkDefinition, bool)
}

// BlockchainClientProvider defines the interface for providing blockchain clients.
type BlockchainClientProvider interface {
	GetClient(networkDefinition entity.NetworkDefinition) (BlockchainClient, error)
}
