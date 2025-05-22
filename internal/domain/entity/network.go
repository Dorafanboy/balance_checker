package entity

// NetworkDefinition holds the configuration for a specific blockchain network.
type NetworkDefinition struct {
	ChainID                   uint64   `json:"chainId" yaml:"chainId"`
	Name                      string   `json:"name" yaml:"name"`
	Identifier                string   `json:"identifier" yaml:"identifier"`
	NativeSymbol              string   `json:"nativeSymbol" yaml:"nativeSymbol"`
	Decimals                  int32    `json:"decimals" yaml:"decimals"`
	PrimaryRPCURL             string   `json:"primaryRpcUrl" yaml:"primaryRpcUrl"`
	FallbackRPCURLs           []string `json:"fallbackRpcUrls" yaml:"fallbackRpcUrls"`
	BlockExplorerURL          string   `json:"blockExplorerUrl,omitempty" yaml:"blockExplorerUrl,omitempty"`
	DEXScreenerChainID        string   `json:"dexScreenerChainId,omitempty" yaml:"dexScreenerChainId,omitempty"`
	WrappedNativeTokenAddress string   `json:"wrappedNativeTokenAddress,omitempty" yaml:"wrappedNativeTokenAddress,omitempty"`
}
