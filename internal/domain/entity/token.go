package entity

// TokenInfo holds the details of a specific token.
type TokenInfo struct {
	ChainID  uint64
	Address  string
	Name     string
	Symbol   string
	Decimals uint8
}
