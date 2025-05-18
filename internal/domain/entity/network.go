package entity

// NetworkDefinition holds the configuration for a specific blockchain network.
// This structure is defined at the domain level to be used across application and infrastructure layers.
type NetworkDefinition struct {
	ChainID          uint64   `json:"chainId" yaml:"chainId"`
	Name             string   `json:"name" yaml:"name"`
	Identifier       string   `json:"identifier" yaml:"identifier"` // Уникальный идентификатор сети (например, "ethereum", "bsc")
	NativeSymbol     string   `json:"nativeSymbol" yaml:"nativeSymbol"`
	Decimals         int32    `json:"decimals" yaml:"decimals"` // Количество десятичных знаков для нативного токена
	PrimaryRPCURL    string   `json:"primaryRpcUrl" yaml:"primaryRpcUrl"`
	FallbackRPCURLs  []string `json:"fallbackRpcUrls" yaml:"fallbackRpcUrls"`
	BlockExplorerURL string   `json:"blockExplorerUrl,omitempty" yaml:"blockExplorerUrl,omitempty"`
	// RPCProviders    map[string]string `json:"rpcProviders,omitempty" yaml:"rpcProviders,omitempty"` // Поле удалено, т.к. RPC теперь часть этой структуры
}
