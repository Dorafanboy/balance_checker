package networkdefinition

import (
	"fmt"
	"os"
	"strings"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
)

// NetworkDefinitionProvider provides network definitions.
type NetworkDefinitionProvider struct {
	logger            port.Logger
	allNetworkDefs    map[string]entity.NetworkDefinition
	activeNetworkDefs []entity.NetworkDefinition
}

// Predefined network definitions
var ( //nolint:gochecknoglobals // Global for definitions
	Ethereum = entity.NetworkDefinition{
		ChainID:                   1,
		Name:                      "Ethereum Mainnet",
		Identifier:                "ethereum",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://ethereum-rpc.publicnode.com",
		FallbackRPCURLs:           []string{"https://rpc.ankr.com/eth", "https://ethereum.publicnode.com"},
		BlockExplorerURL:          "https://etherscan.io",
		DEXScreenerChainID:        "ethereum",
		WrappedNativeTokenAddress: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
	}
	BSC = entity.NetworkDefinition{
		ChainID:                   56,
		Name:                      "BNB Smart Chain",
		Identifier:                "bsc",
		NativeSymbol:              "BNB",
		Decimals:                  18,
		PrimaryRPCURL:             "https://1rpc.io/bnb",
		FallbackRPCURLs:           []string{"https://bsc-dataseed2.binance.org/", "https://bsc.publicnode.com"},
		BlockExplorerURL:          "https://bscscan.com",
		DEXScreenerChainID:        "bsc",
		WrappedNativeTokenAddress: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", // WBNB
	}
	Polygon = entity.NetworkDefinition{
		ChainID:                   137,
		Name:                      "Polygon PoS",
		Identifier:                "polygon",
		NativeSymbol:              "MATIC",
		Decimals:                  18,
		PrimaryRPCURL:             "https://polygon-rpc.com/",
		FallbackRPCURLs:           []string{"https://rpc.ankr.com/polygon", "https://polygon.publicnode.com"},
		BlockExplorerURL:          "https://polygonscan.com",
		DEXScreenerChainID:        "polygon",
		WrappedNativeTokenAddress: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270", // WMATIC
	}
	Arbitrum = entity.NetworkDefinition{
		ChainID:                   42161,
		Name:                      "Arbitrum One",
		Identifier:                "arbitrum",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://arb1.arbitrum.io/rpc",
		FallbackRPCURLs:           []string{"https://arbitrum.llamarpc.com", "https://arbitrum.publicnode.com"},
		BlockExplorerURL:          "https://arbiscan.io",
		DEXScreenerChainID:        "arbitrum",
		WrappedNativeTokenAddress: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // WETH on Arbitrum
	}
	Avalanche = entity.NetworkDefinition{
		ChainID:                   43114,
		Name:                      "Avalanche C-Chain",
		Identifier:                "avalanche",
		NativeSymbol:              "AVAX",
		Decimals:                  18,
		PrimaryRPCURL:             "https://api.avax.network/ext/bc/C/rpc",
		FallbackRPCURLs:           []string{"https://avalanche.public-rpc.com", "https://rpc.ankr.com/avalanche"},
		BlockExplorerURL:          "https://snowtrace.io",
		DEXScreenerChainID:        "avalanche",
		WrappedNativeTokenAddress: "0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7", // WAVAX
	}
	Base = entity.NetworkDefinition{
		ChainID:                   8453,
		Name:                      "Base Mainnet",
		Identifier:                "base",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://1rpc.io/base",
		FallbackRPCURLs:           []string{"https://base.publicnode.com", "https://base.llamarpc.com"},
		BlockExplorerURL:          "https://basescan.org",
		DEXScreenerChainID:        "base",
		WrappedNativeTokenAddress: "0x4200000000000000000000000000000000000006", // WETH on Base
	}
	Blast = entity.NetworkDefinition{
		ChainID:                   81457,
		Name:                      "Blast Mainnet",
		Identifier:                "blast",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://rpc.ankr.com/blast",
		FallbackRPCURLs:           []string{"https://blast.blockpi.network/v1/rpc/public", "https://blastl2-mainnet.public.blastapi.io"},
		BlockExplorerURL:          "https://blastscan.io",
		DEXScreenerChainID:        "blast",                                      // From config comments
		WrappedNativeTokenAddress: "0x4300000000000000000000000000000000000004", // WETH on Blast (проверьте этот адрес, может отличаться)
	}
	Celo = entity.NetworkDefinition{
		ChainID:                   42220,
		Name:                      "Celo Mainnet",
		Identifier:                "celo",
		NativeSymbol:              "CELO",
		Decimals:                  18,
		PrimaryRPCURL:             "https://rpc.ankr.com/celo",
		FallbackRPCURLs:           []string{"https://rpc.ankr.com/celo"},
		BlockExplorerURL:          "https://celoscan.io",
		DEXScreenerChainID:        "celo",                                       // From config comments
		WrappedNativeTokenAddress: "0x471ece3750da237f93b8e339c536989b8978a438", // CELO itself
	}
	Core = entity.NetworkDefinition{
		ChainID:                   1116,
		Name:                      "Core Blockchain",
		Identifier:                "core",
		NativeSymbol:              "CORE",
		Decimals:                  18,
		PrimaryRPCURL:             "https://rpc.coredao.org",
		FallbackRPCURLs:           []string{"https://rpc.ankr.com/core_dao"},
		BlockExplorerURL:          "https://scan.coredao.org",
		DEXScreenerChainID:        "core",                                       // From config comments
		WrappedNativeTokenAddress: "0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000", // WCORE - официальный адрес пока не нашел, это плейсхолдер. НАЙДИТЕ АКТУАЛЬНЫЙ!
	}
	Fantom = entity.NetworkDefinition{
		ChainID:                   250,
		Name:                      "Fantom Opera",
		Identifier:                "fantom",
		NativeSymbol:              "FTM",
		Decimals:                  18,
		PrimaryRPCURL:             "https://1rpc.io/ftm",
		FallbackRPCURLs:           []string{"https://fantom.publicnode.com", "https://rpc.ankr.com/fantom"},
		BlockExplorerURL:          "https://ftmscan.com",
		DEXScreenerChainID:        "fantom",
		WrappedNativeTokenAddress: "0x21be370D5312f44cB42ce377BC9b8a0cEF1A4C83", // WFTM
	}
	Gnosis = entity.NetworkDefinition{
		ChainID:                   100,
		Name:                      "Gnosis Chain",
		Identifier:                "gnosis",
		NativeSymbol:              "xDAI",
		Decimals:                  18,
		PrimaryRPCURL:             "https://0xrpc.io/gno",
		FallbackRPCURLs:           []string{"https://rpc.ankr.com/gnosis", "https://gnosis.publicnode.com"},
		BlockExplorerURL:          "https://gnosisscan.io",
		DEXScreenerChainID:        "gnosis",                                     // From config comments
		WrappedNativeTokenAddress: "0xe91D153E0b41518A2Ce8DD3D7944Fa863463A97d", // WXDAI
	}
	Linea = entity.NetworkDefinition{
		ChainID:                   59144,
		Name:                      "Linea Mainnet",
		Identifier:                "linea",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://rpc.linea.build",
		FallbackRPCURLs:           []string{"https://linea.blockpi.network/v1/rpc/public"},
		BlockExplorerURL:          "https://lineascan.build",
		DEXScreenerChainID:        "linea",                                      // From config comments
		WrappedNativeTokenAddress: "0xe5D7C2a44FfDDf6b295A15c148167daaAf5Cf34f", // WETH on Linea
	}
	Manta = entity.NetworkDefinition{ // Manta Pacific
		ChainID:                   169,
		Name:                      "Manta Pacific Mainnet",
		Identifier:                "manta",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://pacific-rpc.manta.network/http",
		FallbackRPCURLs:           []string{},
		BlockExplorerURL:          "https://pacific-explorer.manta.network",
		DEXScreenerChainID:        "manta",                                      // From config comments
		WrappedNativeTokenAddress: "0x0Dc808AdCeDb63Fc4B9Ac0A9865d50A052A0d5c5", // WETH on Manta (USDC address for WETH? Check this) - ЗАМЕНИТЬ НА АДРЕС WETH!
	}
	Mantle = entity.NetworkDefinition{
		ChainID:                   5000,
		Name:                      "Mantle Network",
		Identifier:                "mantle",
		NativeSymbol:              "MNT",
		Decimals:                  18,
		PrimaryRPCURL:             "https://rpc.mantle.xyz",
		FallbackRPCURLs:           []string{},
		BlockExplorerURL:          "https://explorer.mantle.xyz",
		DEXScreenerChainID:        "mantle",                                     // From config comments
		WrappedNativeTokenAddress: "0x78c1b0C915c4FAA5FffA6CAbf0219DA63d7f4cb8", // WMNT (Wrapped MNT)
	}
	Metis = entity.NetworkDefinition{
		ChainID:                   1088,
		Name:                      "Metis Andromeda Mainnet",
		Identifier:                "metis",
		NativeSymbol:              "METIS",
		Decimals:                  18,
		PrimaryRPCURL:             "https://andromeda.metis.io/?owner=1088",
		FallbackRPCURLs:           []string{},
		BlockExplorerURL:          "https://andromeda-explorer.metis.io",
		DEXScreenerChainID:        "metis",                                      // From config comments
		WrappedNativeTokenAddress: "0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000", // WMetis - НАЙДИТЕ АКТУАЛЬНЫЙ!
	}
	Optimism = entity.NetworkDefinition{
		ChainID:                   10,
		Name:                      "OP Mainnet",
		Identifier:                "optimism",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://op-pokt.nodies.app",
		FallbackRPCURLs:           []string{"https://optimism.publicnode.com", "https://rpc.ankr.com/optimism"},
		BlockExplorerURL:          "https://optimistic.etherscan.io",
		DEXScreenerChainID:        "optimism",
		WrappedNativeTokenAddress: "0x4200000000000000000000000000000000000006", // WETH on Optimism
	}
	PolygonZkEVM = entity.NetworkDefinition{
		ChainID:                   1101,
		Name:                      "Polygon zkEVM",
		Identifier:                "polygon_zkevm",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://zkevm-rpc.com",
		FallbackRPCURLs:           []string{"https://rpc.ankr.com/polygon_zkevm"},
		BlockExplorerURL:          "https://zkevm.polygonscan.com",
		DEXScreenerChainID:        "polygonzkevm",                               // From config comments, needs verification
		WrappedNativeTokenAddress: "0x4F9A0e7FD2Bf6067db6994CF12E4495Df938E6e9", // WETH on Polygon zkEVM
	}
	Scroll = entity.NetworkDefinition{
		ChainID:                   534352,
		Name:                      "Scroll",
		Identifier:                "scroll",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://rpc.scroll.io",
		FallbackRPCURLs:           []string{"https://scroll.blockpi.network/v1/rpc/public"},
		BlockExplorerURL:          "https://scrollscan.com",
		DEXScreenerChainID:        "scroll",                                     // From config comments
		WrappedNativeTokenAddress: "0x5300000000000000000000000000000000000004", // WETH on Scroll
	}
	ZkSync = entity.NetworkDefinition{ // zkSync Era
		ChainID:                   324,
		Name:                      "zkSync Era Mainnet",
		Identifier:                "zksync",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://mainnet.era.zksync.io",
		FallbackRPCURLs:           []string{},
		BlockExplorerURL:          "https://explorer.zksync.io",
		DEXScreenerChainID:        "zksync",
		WrappedNativeTokenAddress: "0x5AEa5775959fBC2557Cc8789bC1bf90A239D9a91", // WETH on zkSync Era
	}
	Zora = entity.NetworkDefinition{
		ChainID:                   7777777,
		Name:                      "Zora Mainnet",
		Identifier:                "zora",
		NativeSymbol:              "ETH",
		Decimals:                  18,
		PrimaryRPCURL:             "https://zora.drpc.org",
		FallbackRPCURLs:           []string{"https://rpc.zora.energy", "https://1rpc.io/zora"},
		BlockExplorerURL:          "https://explorer.zora.energy",
		DEXScreenerChainID:        "zora",                                       // From config comments
		WrappedNativeTokenAddress: "0x4200000000000000000000000000000000000006", // WETH on Zora (стандартный адрес для многих L2)
	}
)

// allKnownDefinitions is a helper to quickly access all hardcoded definitions.
var allKnownDefinitions = map[string]entity.NetworkDefinition{
	Ethereum.Identifier:     Ethereum,
	BSC.Identifier:          BSC,
	Polygon.Identifier:      Polygon,
	Arbitrum.Identifier:     Arbitrum,
	Avalanche.Identifier:    Avalanche,
	Base.Identifier:         Base,
	Blast.Identifier:        Blast,
	Celo.Identifier:         Celo,
	Core.Identifier:         Core,
	Fantom.Identifier:       Fantom,
	Gnosis.Identifier:       Gnosis,
	Linea.Identifier:        Linea,
	Manta.Identifier:        Manta,
	Mantle.Identifier:       Mantle,
	Metis.Identifier:        Metis,
	Optimism.Identifier:     Optimism,
	PolygonZkEVM.Identifier: PolygonZkEVM,
	Scroll.Identifier:       Scroll,
	ZkSync.Identifier:       ZkSync,
	Zora.Identifier:         Zora,
}

// NewNetworkDefinitionProvider creates a new NetworkDefinitionProvider.
func NewNetworkDefinitionProvider(log port.Logger, tokenDataDir string) *NetworkDefinitionProvider {
	p := &NetworkDefinitionProvider{
		logger:            log,
		allNetworkDefs:    allKnownDefinitions,
		activeNetworkDefs: make([]entity.NetworkDefinition, 0),
	}

	files, err := os.ReadDir(tokenDataDir)
	if err != nil {
		p.logger.Error(fmt.Sprintf("Failed to read token data directory: %s", tokenDataDir), "error", err)
		return p
	}

	activeIdentifiers := make(map[string]struct{})

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".json") {
			continue
		}

		identifier := strings.TrimSuffix(strings.ToLower(file.Name()), ".json")

		if _, alreadyActive := activeIdentifiers[identifier]; alreadyActive {
			p.logger.Warn(fmt.Sprintf("Duplicate token file or identifier detected (after .json trim and lowercasing): %s. Skipping.", identifier))
			continue
		}

		def, ok := p.allNetworkDefs[identifier]
		if !ok {
			p.logger.Warn(fmt.Sprintf("Token file found for network '%s' but no corresponding hardcoded network definition exists. Skipping.", identifier))
			continue
		}

		p.activeNetworkDefs = append(p.activeNetworkDefs, def)
		activeIdentifiers[identifier] = struct{}{}
		p.logger.Debug(fmt.Sprintf("Network '%s' activated due to presence of token file '%s'.", def.Name, file.Name()))
	}

	if len(p.activeNetworkDefs) == 0 {
		p.logger.Warn("No token files found or no matching network definitions for token files in directory. No networks will be active.", "directory", tokenDataDir)
	} else {
		p.logger.Info(fmt.Sprintf("NetworkDefinitionProvider initialized. Active networks: %d (determined by token files)", len(p.activeNetworkDefs)))
		for _, netDef := range p.activeNetworkDefs {
			p.logger.Debug(fmt.Sprintf("  - Active network: %s (ID: %s, ChainID: %d, DEXScreenerID: %s)", netDef.Name, netDef.Identifier, netDef.ChainID, netDef.DEXScreenerChainID))
		}
	}

	return p
}

// GetAllNetworkDefinitions returns the list of active (tracked) network definitions.
func (p *NetworkDefinitionProvider) GetAllNetworkDefinitions() []entity.NetworkDefinition {
	if p == nil {
		return []entity.NetworkDefinition{}
	}
	defsCopy := make([]entity.NetworkDefinition, len(p.activeNetworkDefs))
	copy(defsCopy, p.activeNetworkDefs)
	return defsCopy
}

// GetNetworkDefinitionByName returns a specific network definition by its identifier if it's active.
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

	for _, knownDef := range p.allNetworkDefs {
		if knownDef.ChainID == chainID {
			p.logger.Warn(fmt.Sprintf("Network with ChainID %d found in all definitions but not in active tracked list.", chainID))
			return knownDef, true
		}
	}

	return entity.NetworkDefinition{}, false
}
