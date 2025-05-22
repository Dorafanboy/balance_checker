package client

import (
	"fmt"
	"sync"
	"time"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/infrastructure/configloader"
)

const (
	defaultProviderConnectionTimeout = 10 * time.Second
)

// evmClientProvider implements the port.BlockchainClientProvider interface.
type evmClientProvider struct {
	clients           map[string]port.BlockchainClient
	mu                sync.Mutex
	loggerInfo        func(msg string, args ...any)
	loggerError       func(msg string, args ...any)
	connectionTimeout time.Duration
	rpcCallTimeout    time.Duration
}

// NewEVMClientProvider creates a new EVMClientProvider.
func NewEVMClientProvider(
	cfg *configloader.Config,
	loggerInfo func(msg string, args ...any),
	loggerError func(msg string, args ...any),
) port.BlockchainClientProvider {
	return &evmClientProvider{
		clients:           make(map[string]port.BlockchainClient),
		loggerInfo:        loggerInfo,
		loggerError:       loggerError,
		connectionTimeout: defaultProviderConnectionTimeout,
		rpcCallTimeout:    time.Duration(cfg.Performance.RPCCallTimeoutSeconds) * time.Second,
	}
}

// GetClient retrieves a blockchain client for the given network definition.
func (p *evmClientProvider) GetClient(netDef entity.NetworkDefinition) (port.BlockchainClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	clientKey := netDef.Name
	if client, exists := p.clients[clientKey]; exists {
		p.loggerInfo("Returning cached EVM client", "network", netDef.Name)
		return client, nil
	}

	p.loggerInfo("Creating new EVM client", "network", netDef.Name, "rpc_primary", netDef.PrimaryRPCURL)
	newClient, err := NewEVMClient(netDef, nil, p.connectionTimeout, p.rpcCallTimeout)
	if err != nil {
		p.loggerError("Failed to create EVM client", "network", netDef.Name, "error", err)
		return nil, fmt.Errorf("failed to create EVM client for %s: %w", netDef.Name, err)
	}

	p.clients[clientKey] = newClient
	p.loggerInfo("Successfully created and cached new EVM client", "network", netDef.Name)
	return newClient, nil
}

// GetAllClients returns all currently cached clients.
func (p *evmClientProvider) GetAllClients() map[string]port.BlockchainClient {
	p.mu.Lock()
	defer p.mu.Unlock()

	copiedClients := make(map[string]port.BlockchainClient, len(p.clients))
	for k, v := range p.clients {
		copiedClients[k] = v
	}
	return copiedClients
}
