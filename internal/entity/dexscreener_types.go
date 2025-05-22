package entity

// DEXTokenPair represents a single trading pair data from DEX Screener.
type DEXTokenPair struct {
	SchemaVersion string     `json:"schemaVersion"`
	Pair          *PairData  `json:"pair"`  // Adjusted based on common single pair responses, if the /tokens/v1 endpoint returns an array of these, the client will handle it.
	Pairs         []PairData `json:"pairs"` // Used if the endpoint returns multiple pairs, e.g. for a search or by token address
}

// PairData contains detailed information about a trading pair.
// This structure might be nested if the API returns a single object with a "pair" key,
// or it might be the direct element in an array if "pairs" key is used.
type PairData struct {
	ChainID       string          `json:"chainId"`
	DexID         string          `json:"dexId"`
	URL           string          `json:"url"`
	PairAddress   string          `json:"pairAddress"`
	BaseToken     DEXToken        `json:"baseToken"`
	QuoteToken    DEXToken        `json:"quoteToken"`
	PriceNative   string          `json:"priceNative"`
	PriceUsd      string          `json:"priceUsd"`
	Txns          PairTxns        `json:"txns"`
	Volume        PairVolume      `json:"volume"`
	PriceChange   PairPriceChange `json:"priceChange"`
	Liquidity     *DEXLiquidity   `json:"liquidity"` // Pointer to handle potential nulls
	Fdv           float64         `json:"fdv"`
	MarketCap     float64         `json:"marketCap"`
	PairCreatedAt int64           `json:"pairCreatedAt"`
	// Info can be more complex, define if needed
}

// DEXToken represents a token in a trading pair.
type DEXToken struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Symbol  string `json:"symbol"`
}

// DEXLiquidity represents the liquidity information for a pair.
type DEXLiquidity struct {
	Usd   float64 `json:"usd"`
	Base  float64 `json:"base"`
	Quote float64 `json:"quote"`
}

// PairTxns represents transaction counts for a pair.
type PairTxns struct {
	M5  TxnSummary `json:"m5"`
	H1  TxnSummary `json:"h1"`
	H6  TxnSummary `json:"h6"`
	H24 TxnSummary `json:"h24"`
}

// TxnSummary contains buy and sell counts.
type TxnSummary struct {
	Buys  int `json:"buys"`
	Sells int `json:"sells"`
}

// PairVolume represents trading volume over different periods.
type PairVolume struct {
	M5  float64 `json:"m5"`
	H1  float64 `json:"h1"`
	H6  float64 `json:"h6"`
	H24 float64 `json:"h24"`
}

// PairPriceChange represents price change percentage over different periods.
type PairPriceChange struct {
	M5  float64 `json:"m5"`
	H1  float64 `json:"h1"`
	H6  float64 `json:"h6"`
	H24 float64 `json:"h24"`
}
