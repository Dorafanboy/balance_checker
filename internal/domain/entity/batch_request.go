package entity

import "math/big"

// BalanceRequestType defines the type of balance request.
type BalanceRequestType int

const (
	// NativeBalanceRequest requests the native balance of a wallet.
	NativeBalanceRequest BalanceRequestType = iota
	// TokenBalanceRequest requests the balance of a specific token for a wallet.
	TokenBalanceRequest
)

// ZeroAddress represents the Ethereum zero address.
const ZeroAddress = "0x0000000000000000000000000000000000000000"

// BalanceRequestItem represents a single item in a batch request for balances.
type BalanceRequestItem struct {
	ID            string
	Type          BalanceRequestType
	WalletAddress string
	TokenAddress  string
	TokenSymbol   string
	TokenDecimals uint8
}

// BalanceResultItem represents the result of a single balance request from a batch.
type BalanceResultItem struct {
	RequestID        string
	WalletAddress    string
	NetworkName      string
	ChainID          string
	TokenAddress     string
	TokenSymbol      string
	Decimals         uint8
	IsNative         bool
	Balance          *big.Int
	FormattedBalance string
	Error            error
}
