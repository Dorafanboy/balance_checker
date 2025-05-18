package networkdefinition

import (
	"fmt"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
)

// NetworkDefinitionProvider provides network definitions.
type NetworkDefinitionProvider struct {
	logger            port.Logger
	allNetworkDefs    map[string]entity.NetworkDefinition // Карта для быстрого доступа по Identifier
	activeNetworkDefs []entity.NetworkDefinition
}

// Predefined network definitions
var ( //nolint:gochecknoglobals // Global for definitions
	Ethereum = entity.NetworkDefinition{
		ChainID:          1,
		Name:             "Ethereum Mainnet",
		Identifier:       "ethereum",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://ethereum-rpc.publicnode.com",
		FallbackRPCURLs:  []string{"https://rpc.ankr.com/eth", "https://ethereum.publicnode.com"},
		BlockExplorerURL: "https://etherscan.io",
	}
	BSC = entity.NetworkDefinition{
		ChainID:          56,
		Name:             "BNB Smart Chain",
		Identifier:       "bsc",
		NativeSymbol:     "BNB",
		Decimals:         18,
		PrimaryRPCURL:    "https://bsc.drpc.org",
		FallbackRPCURLs:  []string{"https://bsc-dataseed2.binance.org/", "https://bsc.publicnode.com"},
		BlockExplorerURL: "https://bscscan.com",
	}
	Polygon = entity.NetworkDefinition{
		ChainID:          137,
		Name:             "Polygon PoS",
		Identifier:       "polygon",
		NativeSymbol:     "MATIC",
		Decimals:         18,
		PrimaryRPCURL:    "https://polygon-rpc.com/",
		FallbackRPCURLs:  []string{"https://rpc.ankr.com/polygon", "https://polygon.publicnode.com"},
		BlockExplorerURL: "https://polygonscan.com",
	}
	Arbitrum = entity.NetworkDefinition{
		ChainID:          42161,
		Name:             "Arbitrum One",
		Identifier:       "arbitrum",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://arb1.arbitrum.io/rpc",
		FallbackRPCURLs:  []string{"https://arbitrum.llamarpc.com", "https://arbitrum.publicnode.com"},
		BlockExplorerURL: "https://arbiscan.io",
	}
	Avalanche = entity.NetworkDefinition{
		ChainID:          43114,
		Name:             "Avalanche C-Chain",
		Identifier:       "avalanche",
		NativeSymbol:     "AVAX",
		Decimals:         18,
		PrimaryRPCURL:    "https://api.avax.network/ext/bc/C/rpc",
		FallbackRPCURLs:  []string{"https://avalanche.public-rpc.com", "https://rpc.ankr.com/avalanche"},
		BlockExplorerURL: "https://snowtrace.io",
	}
	Base = entity.NetworkDefinition{
		ChainID:          8453,
		Name:             "Base Mainnet",
		Identifier:       "base",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://mainnet.base.org",
		FallbackRPCURLs:  []string{"https://base.publicnode.com", "https://base.llamarpc.com"},
		BlockExplorerURL: "https://basescan.org",
	}
	Blast = entity.NetworkDefinition{
		ChainID:          81457,
		Name:             "Blast Mainnet",
		Identifier:       "blast",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://blast.drpc.org",
		FallbackRPCURLs:  []string{"https://blast.blockpi.network/v1/rpc/public", "https://blastl2-mainnet.public.blastapi.io"},
		BlockExplorerURL: "https://blastscan.io",
	}
	Celo = entity.NetworkDefinition{
		ChainID:          42220,
		Name:             "Celo Mainnet",
		Identifier:       "celo",
		NativeSymbol:     "CELO",
		Decimals:         18,
		PrimaryRPCURL:    "https://celo.drpc.org",
		FallbackRPCURLs:  []string{"https://rpc.ankr.com/celo"},
		BlockExplorerURL: "https://celoscan.io",
	}
	Core = entity.NetworkDefinition{
		ChainID:          1116,
		Name:             "Core Blockchain",
		Identifier:       "core",
		NativeSymbol:     "CORE",
		Decimals:         18,
		PrimaryRPCURL:    "https://rpc.coredao.org",
		FallbackRPCURLs:  []string{"https://rpc.ankr.com/core_dao"},
		BlockExplorerURL: "https://scan.coredao.org",
	}
	Fantom = entity.NetworkDefinition{
		ChainID:          250,
		Name:             "Fantom Opera",
		Identifier:       "fantom",
		NativeSymbol:     "FTM",
		Decimals:         18,
		PrimaryRPCURL:    "https://1rpc.io/ftm",
		FallbackRPCURLs:  []string{"https://fantom.publicnode.com", "https://rpc.ankr.com/fantom"},
		BlockExplorerURL: "https://ftmscan.com",
	}
	Gnosis = entity.NetworkDefinition{
		ChainID:          100,
		Name:             "Gnosis Chain",
		Identifier:       "gnosis",
		NativeSymbol:     "xDAI",
		Decimals:         18,
		PrimaryRPCURL:    "https://gnosis.drpc.org",
		FallbackRPCURLs:  []string{"https://rpc.ankr.com/gnosis", "https://gnosis.publicnode.com"},
		BlockExplorerURL: "https://gnosisscan.io",
	}
	Linea = entity.NetworkDefinition{
		ChainID:          59144,
		Name:             "Linea Mainnet",
		Identifier:       "linea",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://rpc.linea.build",
		FallbackRPCURLs:  []string{"https://linea.blockpi.network/v1/rpc/public"},
		BlockExplorerURL: "https://lineascan.build",
	}
	Manta = entity.NetworkDefinition{ // Manta Pacific
		ChainID:          169,
		Name:             "Manta Pacific Mainnet",
		Identifier:       "manta",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://pacific-rpc.manta.network/http",
		FallbackRPCURLs:  []string{},
		BlockExplorerURL: "https://pacific-explorer.manta.network",
	}
	Mantle = entity.NetworkDefinition{
		ChainID:          5000,
		Name:             "Mantle Network",
		Identifier:       "mantle",
		NativeSymbol:     "MNT",
		Decimals:         18,
		PrimaryRPCURL:    "https://rpc.mantle.xyz",
		FallbackRPCURLs:  []string{},
		BlockExplorerURL: "https://explorer.mantle.xyz",
	}
	Metis = entity.NetworkDefinition{
		ChainID:          1088,
		Name:             "Metis Andromeda Mainnet",
		Identifier:       "metis",
		NativeSymbol:     "METIS",
		Decimals:         18,
		PrimaryRPCURL:    "https://andromeda.metis.io/?owner=1088",
		FallbackRPCURLs:  []string{},
		BlockExplorerURL: "https://andromeda-explorer.metis.io",
	}
	Optimism = entity.NetworkDefinition{
		ChainID:          10,
		Name:             "OP Mainnet",
		Identifier:       "optimism",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://op-pokt.nodies.app",
		FallbackRPCURLs:  []string{"https://optimism.publicnode.com", "https://rpc.ankr.com/optimism"},
		BlockExplorerURL: "https://optimistic.etherscan.io",
	}
	PolygonZkEVM = entity.NetworkDefinition{
		ChainID:          1101,
		Name:             "Polygon zkEVM",
		Identifier:       "polygon_zkevm",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://zkevm-rpc.com",
		FallbackRPCURLs:  []string{"https://rpc.ankr.com/polygon_zkevm"},
		BlockExplorerURL: "https://zkevm.polygonscan.com",
	}
	Scroll = entity.NetworkDefinition{
		ChainID:          534352,
		Name:             "Scroll",
		Identifier:       "scroll",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://rpc.scroll.io",
		FallbackRPCURLs:  []string{"https://scroll.blockpi.network/v1/rpc/public"},
		BlockExplorerURL: "https://scrollscan.com",
	}
	ZkSync = entity.NetworkDefinition{ // zkSync Era
		ChainID:          324,
		Name:             "zkSync Era Mainnet",
		Identifier:       "zksync",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://mainnet.era.zksync.io",
		FallbackRPCURLs:  []string{},
		BlockExplorerURL: "https://explorer.zksync.io",
	}
	Zora = entity.NetworkDefinition{
		ChainID:          7777777,
		Name:             "Zora Mainnet",
		Identifier:       "zora",
		NativeSymbol:     "ETH",
		Decimals:         18,
		PrimaryRPCURL:    "https://zora.drpc.org",
		FallbackRPCURLs:  []string{},
		BlockExplorerURL: "https://explorer.zora.energy",
	}
	/* // УДАЛЕНО ZetaChain
	ZetaChain = entity.NetworkDefinition{
		ChainID:          7000,
		Name:             "ZetaChain Mainnet",
		Identifier:       "zetachain",
		NativeSymbol:     "ZETA",
		Decimals:         18,
		PrimaryRPCURL:    "https://api.mainnet.zetachain.com/evm",
		FallbackRPCURLs:  []string{},
		BlockExplorerURL: "https://explorer.mainnet.zetachain.com",
	}
	*/
)

// allKnownDefinitions is a helper to quickly access all hardcoded definitions.
// It's initialized once.
var allKnownDefinitions = map[string]entity.NetworkDefinition{
	Ethereum.Identifier:  Ethereum,
	BSC.Identifier:       BSC,
	Polygon.Identifier:   Polygon,
	Arbitrum.Identifier:  Arbitrum,
	Avalanche.Identifier: Avalanche,
	Base.Identifier:      Base,
	Blast.Identifier:     Blast,
	Celo.Identifier:      Celo,
	Core.Identifier:      Core,
	Fantom.Identifier:    Fantom,
	Gnosis.Identifier:    Gnosis,
	Linea.Identifier:     Linea,
	Manta.Identifier:     Manta,
	Mantle.Identifier:    Mantle,
	Metis.Identifier:     Metis,
	// Mode.Identifier:         Mode, // УДАЛЕНО
	Optimism.Identifier:     Optimism,
	PolygonZkEVM.Identifier: PolygonZkEVM,
	Scroll.Identifier:       Scroll,
	ZkSync.Identifier:       ZkSync,
	Zora.Identifier:         Zora,
	// ZetaChain.Identifier:    ZetaChain, // УДАЛЕНО
}

// NewNetworkDefinitionProvider creates a new NetworkDefinitionProvider.
// It filters allKnownDefinitions based on the provided trackedNetworkIdentifiers from the config.
func NewNetworkDefinitionProvider(log port.Logger, trackedNetworkIdentifiers []string) *NetworkDefinitionProvider {
	p := &NetworkDefinitionProvider{
		logger:            log,
		allNetworkDefs:    allKnownDefinitions,
		activeNetworkDefs: make([]entity.NetworkDefinition, 0, len(trackedNetworkIdentifiers)),
	}

	if len(trackedNetworkIdentifiers) == 0 {
		p.logger.Warn("No networks configured in 'tracked_networks'. Service might not be able to fetch any balances.")
		return p
	}

	configuredIdentifiers := make(map[string]struct{}) // For quick lookup and duplicate check
	for _, id := range trackedNetworkIdentifiers {
		if _, exists := configuredIdentifiers[id]; exists {
			p.logger.Warn(fmt.Sprintf("Network identifier '%s' is duplicated in config, will be processed once.", id))
			continue
		}
		configuredIdentifiers[id] = struct{}{}

		def, ok := p.allNetworkDefs[id]
		if !ok {
			p.logger.Warn(fmt.Sprintf("No network definition found for configured network identifier: '%s'. Skipping.", id))
			continue
		}
		p.activeNetworkDefs = append(p.activeNetworkDefs, def)
	}

	if len(p.activeNetworkDefs) == 0 {
		p.logger.Warn("After filtering, no valid network definitions are active. Please check 'tracked_networks' in config and available definitions.")
	} else {
		p.logger.Info(fmt.Sprintf("NetworkDefinitionProvider initialized. Active networks: %d", len(p.activeNetworkDefs)))
		for _, netDef := range p.activeNetworkDefs {
			p.logger.Debug(fmt.Sprintf("  - Active network: %s (ID: %s, ChainID: %d)", netDef.Name, netDef.Identifier, netDef.ChainID))
		}
	}

	return p
}

// GetNetworkDefinitions returns the list of active (tracked) network definitions.
func (p *NetworkDefinitionProvider) GetAllNetworkDefinitions() []entity.NetworkDefinition {
	if p == nil {
		return []entity.NetworkDefinition{}
	}
	// Return a copy to prevent external modification
	defsCopy := make([]entity.NetworkDefinition, len(p.activeNetworkDefs))
	copy(defsCopy, p.activeNetworkDefs)
	return defsCopy
}

// GetNetworkDefinitionByIdentifier returns a specific network definition by its identifier if it's active.
func (p *NetworkDefinitionProvider) GetNetworkDefinitionByName(identifier string) (entity.NetworkDefinition, bool) {
	if p == nil {
		return entity.NetworkDefinition{}, false
	}
	for _, def := range p.activeNetworkDefs {
		if def.Identifier == identifier {
			return def, true
		}
	}
	return entity.NetworkDefinition{}, false
}

// GetNetworkDefinitionByChainID returns a specific network definition by its chain ID if it's active.
func (p *NetworkDefinitionProvider) GetNetworkDefinitionByChainID(chainID uint64) (entity.NetworkDefinition, bool) {
	if p == nil {
		return entity.NetworkDefinition{}, false
	}
	for _, def := range p.activeNetworkDefs {
		if def.ChainID == chainID {
			return def, true
		}
	}

	// Fallback to check all known definitions if not found in active, but log a warning if found this way.
	// This might be useful for some internal lookups but should be used cautiously.
	// def, ok := p.allNetworkDefs[strconv.FormatUint(chainID, 10)] // Note: allNetworkDefs is keyed by Identifier.
	// This specific fallback won't work directly with chainID unless map is keyed differently or we iterate.
	// For now, let's iterate allKnownDefinitions if not found in active for this specific function, though it's less efficient.
	for _, knownDef := range p.allNetworkDefs {
		if knownDef.ChainID == chainID {
			p.logger.Warn(fmt.Sprintf("Network with ChainID %d found in all definitions but not in active tracked list.", chainID))
			return knownDef, true // Or decide if this should return false
		}
	}

	return entity.NetworkDefinition{}, false
}
