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
	ID            string // Unique identifier for the request (optional, can be used for correlation)
	Type          BalanceRequestType
	WalletAddress string
	TokenAddress  string // Empty for native balance requests
	TokenSymbol   string // For logging and error reporting
	TokenDecimals uint8  // Needed for formatting the raw balance if not done by the client
}

// BalanceResultItem represents the result of a single balance request from a batch.
type BalanceResultItem struct {
	RequestID        string   // Corresponds to BalanceRequestItem.ID
	WalletAddress    string   // From the original request
	NetworkName      string   // Network name where the balance was fetched (to be filled by service layer)
	ChainID          string   // ChainID of the network (to be filled by service layer)
	TokenAddress     string   // From the original request (empty for native)
	TokenSymbol      string   // From the original request
	Decimals         uint8    // From the original request
	IsNative         bool     // True if this was a native balance request
	Balance          *big.Int // Raw balance amount
	FormattedBalance string   // Formatted balance string
	Error            error    // Error specific to this sub-request
}
