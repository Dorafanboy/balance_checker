package entity

import "math/big"

// Balance represents the amount of a specific token held by a wallet on a network.
type Balance struct {
	WalletAddress    string   `json:"walletAddress" yaml:"walletAddress"`
	NetworkName      string   `json:"networkName" yaml:"networkName"`   // Renamed from NetworkShortName
	ChainID          string   `json:"chainId" yaml:"chainId"`           // Added
	TokenAddress     string   `json:"tokenAddress" yaml:"tokenAddress"` // Added (e.g. "NATIVE" for native token)
	TokenSymbol      string   `json:"tokenSymbol" yaml:"tokenSymbol"`
	Decimals         uint8    `json:"decimals" yaml:"decimals"`                 // Number of decimals for the token
	IsNative         bool     `json:"isNative" yaml:"isNative"`                 // Added: true if this is the native network token
	Amount           *big.Int `json:"amount" yaml:"amount"`                     // Raw balance (smallest unit)
	FormattedBalance string   `json:"formattedBalance" yaml:"formattedBalance"` // Human-readable balance (was FormattedAmount)
}
