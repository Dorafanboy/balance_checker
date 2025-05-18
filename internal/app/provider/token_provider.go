package provider

import (
	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/infrastructure/network/definition"
	"balance_checker/internal/infrastructure/tokenloader"
)

type tokenProviderImpl struct {
	tokenDir          string
	activeNetworkDefs map[string]definition.NetworkDefinition // Key: NetworkShortName
	logger            port.Logger
	tokensCache       map[string][]entity.TokenInfo // Cache loaded tokens
}

// NewTokenProvider creates a new TokenProvider.
func NewTokenProvider(tokenDir string, activeNetworkDefs map[string]definition.NetworkDefinition, logger port.Logger) port.TokenProvider {
	return &tokenProviderImpl{
		tokenDir:          tokenDir,
		activeNetworkDefs: activeNetworkDefs,
		logger:            logger,
	}
}

// GetTokensByNetwork loads token definitions from JSON files for active networks.
// It caches the results after the first successful load.
func (p *tokenProviderImpl) GetTokensByNetwork() (map[string][]entity.TokenInfo, error) {
	if p.tokensCache != nil {
		p.logger.Debug("Returning cached tokens by network")
		return p.tokensCache, nil
	}

	p.logger.Debug("Loading tokens from disk", "directory", p.tokenDir)
	tokens, err := tokenloader.LoadTokens(p.tokenDir, p.activeNetworkDefs)
	if err != nil {
		p.logger.Error("Failed to load tokens", "directory", p.tokenDir, "error", err)
		return nil, err // Error from LoadTokens (e.g., bad file) is fatal as per plan
	}

	p.tokensCache = tokens
	p.logger.Info("Tokens loaded and cached successfully", "total_networks_with_tokens", len(tokens))
	return tokens, nil
}
