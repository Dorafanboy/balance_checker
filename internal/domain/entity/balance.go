package entity

import "math/big"

// Balance represents the amount of a specific token held by a wallet on a network.
type Balance struct {
	WalletAddress    string   `json:"-" yaml:"walletAddress"`
	NetworkName      string   `json:"networkName" yaml:"networkName"`   // Renamed from NetworkShortName
	ChainID          string   `json:"chainId" yaml:"chainId"`           // Added
	TokenAddress     string   `json:"tokenAddress" yaml:"tokenAddress"` // Added (e.g. "NATIVE" for native token)
	TokenSymbol      string   `json:"tokenSymbol" yaml:"tokenSymbol"`
	Decimals         uint8    `json:"decimals" yaml:"decimals"`                 // Number of decimals for the token
	IsNative         bool     `json:"-" yaml:"isNative"`                        // ИЗМЕНЕНО: Скрыто из JSON
	Amount           *big.Int `json:"-" yaml:"amount"`                          // ИЗМЕНЕНО: Скрыто из JSON
	FormattedBalance string   `json:"formattedBalance" yaml:"formattedBalance"` // Human-readable balance (was FormattedAmount)
}
