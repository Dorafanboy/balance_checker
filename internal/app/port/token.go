package port

import "balance_checker/internal/domain/entity"

// TokenProvider defines the interface for fetching token definitions.
type TokenProvider interface {
	// GetTokensByNetwork returns a map of Network ChainID (as string) to a slice of TokenInfo for active networks.
	GetTokensByNetwork(activeNetworkDefs []entity.NetworkDefinition) (map[string][]entity.TokenInfo, error)
}
