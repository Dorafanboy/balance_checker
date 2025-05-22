package entity

import "math/big"

// Balance represents the amount of a specific token held by a wallet on a network.
type Balance struct {
	WalletAddress    string   `json:"-" yaml:"walletAddress"`
	NetworkName      string   `json:"networkName" yaml:"networkName"`
	ChainID          string   `json:"chainId" yaml:"chainId"`
	TokenAddress     string   `json:"tokenAddress" yaml:"tokenAddress"`
	TokenSymbol      string   `json:"tokenSymbol" yaml:"tokenSymbol"`
	Decimals         uint8    `json:"decimals" yaml:"decimals"`
	IsNative         bool     `json:"-" yaml:"isNative"`
	Amount           *big.Int `json:"-" yaml:"amount"`
	FormattedBalance string   `json:"formattedBalance" yaml:"formattedBalance"`
}
