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
	defaultProviderConnectionTimeout = 10 * time.Second // Można to później przenieść do konfiguracji
)

// evmClientProvider implements the port.BlockchainClientProvider interface.
type evmClientProvider struct {
	clients           map[string]port.BlockchainClient
	mu                sync.Mutex
	loggerInfo        func(msg string, args ...any)
	loggerError       func(msg string, args ...any)
	connectionTimeout time.Duration
	rpcCallTimeout    time.Duration
	// Можно также добавить поле для httpClient, если он будет глобально настраиваться для всех клиентов
	// httpClient *http.Client
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
		connectionTimeout: defaultProviderConnectionTimeout, // This might also come from config if needed
		rpcCallTimeout:    time.Duration(cfg.Performance.RPCCallTimeoutSeconds) * time.Second,
	}
}

// GetClient retrieves a blockchain client for the given network definition.
// It caches clients to avoid reconnecting repeatedly.
func (p *evmClientProvider) GetClient(netDef entity.NetworkDefinition) (port.BlockchainClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	clientKey := netDef.Name // Klucz może być bardziej unikalny, np. ChainID + Name
	if client, exists := p.clients[clientKey]; exists {
		// Тут можно добавить проверку жизнеспособности клиента, если необходимо
		// np. client.IsAlive()
		p.loggerInfo("Returning cached EVM client", "network", netDef.Name)
		return client, nil
	}

	p.loggerInfo("Creating new EVM client", "network", netDef.Name, "rpc_primary", netDef.PrimaryRPCURL)
	// httpClient можно передать nil, если не используем специфичную конфигурацию http клиента в NewEVMClient
	// или если NewEVMClient сам создает/управляет своим http клиентом.
	// Для NewEVMClient, который мы видели, httpClient не используется напрямую в DialContext,
	// но может быть полезен, если бы мы использовали NewClientWithOpts.
	// Пока передадим nil.
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
// This might not be directly part of the port.BlockchainClientProvider interface
// but can be useful for some management tasks.
func (p *evmClientProvider) GetAllClients() map[string]port.BlockchainClient {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Возвращаем копию, чтобы избежать проблем с конкурентным доступом к карте извне
	copiedClients := make(map[string]port.BlockchainClient, len(p.clients))
	for k, v := range p.clients {
		copiedClients[k] = v
	}
	return copiedClients
}
