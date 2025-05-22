package client

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"
	"balance_checker/internal/pkg/utils"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// EVMClient implements the port.BlockchainClient interface for EVM-compatible chains.
type EVMClient struct {
	ethClient      *ethclient.Client
	netDef         entity.NetworkDefinition
	rpcCallTimeout time.Duration // ADDED: Timeout for RPC calls
	// logger    port.Logger // Optional: if specific logging per client is needed
}

// ERC20 ABI minimal part for balanceOf
const erc20ABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]`

var (
	parsedERC20ABI  abi.ABI
	parsedERC20Once sync.Once
	erc20MethodID   []byte
)

func initParsedERC20ABI() {
	parsedERC20Once.Do(func() {
		var err error
		parsedERC20ABI, err = abi.JSON(strings.NewReader(erc20ABI))
		if err != nil {
			// This is a critical error during initialization, panic is appropriate
			panic(fmt.Sprintf("failed to parse ERC20 ABI: %v", err))
		}
		balanceOfMethod, ok := parsedERC20ABI.Methods["balanceOf"]
		if !ok {
			panic("balanceOf method not found in parsed ERC20 ABI")
		}
		erc20MethodID = balanceOfMethod.ID
	})
}

// NewEVMClient creates a new EVM client for the given network definition.
// It tries to connect to the primary RPC URL, then fallbacks if necessary.
func NewEVMClient(netDef entity.NetworkDefinition, httpClient *http.Client, connectionTimeout time.Duration, rpcCallTimeout time.Duration) (port.BlockchainClient, error) {
	initParsedERC20ABI() // Ensure ABI is parsed
	rpcURLs := append([]string{netDef.PrimaryRPCURL}, netDef.FallbackRPCURLs...)
	var lastErr error

	// Custom HTTP client for ethclient to control timeout
	// ethclient.NewClient does not directly expose http.Client timeout settings easily after creation.
	// Instead, we can use DialContext with our custom http client.

	for _, rpcURL := range rpcURLs {
		// Create a context with timeout for the connection attempt itself
		// ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
		// defer cancel()
		// Using DialContext for more control with custom http client
		// Note: ethclient.DialContext uses its own transport, so direct http.Client injection is tricky for Dial.
		// We use standard Dial and rely on underlying net.Dialer timeouts or context for ethclient.

		ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
		// ethclient.DialContext does not use the http.Client passed for the actual websocket/ipc/http transport setup in the same way an http request does.
		// For HTTP based RPCs, it will eventually use an http.Client, but the initial Dial is more generic.
		// To control HTTP client specific settings like TLS or custom headers, you'd use ethclient.NewClientWithOpts with rpc.WithHTTPClient(httpClient)
		// However, for simple timeout, context is the primary way for Dial.

		// Let's use ethclient.Dial for simplicity, context will handle timeout for the dial operation.
		// For more advanced http client options, one might need NewClientWithOpts and rpc.WithHTTPClient.
		// However, `ethclient.NewClient(rpc.NewClient(rawClient))` where `rawClient` is custom can be one way for full http control.

		// client, err := ethclient.Dial(rpcURL) // Original simple way
		client, err := ethclient.DialContext(ctx, rpcURL) // Using context for timeout
		cancel()                                          // Cancel context as soon as dial is done or fails

		if err == nil {
			// Optional: Verify connection with a quick call like ChainID, though not strictly necessary here
			// currentChainID, chainErr := client.ChainID(context.Background())
			// if chainErr != nil { lastErr = fmt.Errorf("failed to verify chainID for %s: %w", rpcURL, chainErr); continue }
			// if currentChainID.Uint64() != netDef.ChainID { lastErr = fmt.Errorf("chainID mismatch for %s: expected %d, got %d", rpcURL, netDef.ChainID, currentChainID.Uint64()); continue }
			return &EVMClient{ethClient: client, netDef: netDef, rpcCallTimeout: rpcCallTimeout}, nil
		}
		lastErr = fmt.Errorf("failed to connect to RPC %s: %w", rpcURL, err)
		// logger.L().Warn("Failed to connect to RPC, trying next", "url", rpcURL, "error", err)
	}

	return nil, fmt.Errorf("all RPC connection attempts failed for network %s: %w", netDef.Name, lastErr)
}

// GetBalances fetches multiple balances using JSON-RPC batch requests.
func (c *EVMClient) GetBalances(ctx context.Context, requests []entity.BalanceRequestItem) ([]entity.BalanceResultItem, error) {
	if len(requests) == 0 {
		return []entity.BalanceResultItem{}, nil
	}

	batchElems := make([]rpc.BatchElem, len(requests))
	results := make([]entity.BalanceResultItem, len(requests))

	for i, reqItem := range requests {
		// Pre-fill result item with request data
		results[i] = entity.BalanceResultItem{
			RequestID:     reqItem.ID,
			WalletAddress: reqItem.WalletAddress,
			TokenAddress:  reqItem.TokenAddress,
			TokenSymbol:   reqItem.TokenSymbol,
			Decimals:      reqItem.TokenDecimals,
			IsNative:      reqItem.Type == entity.NativeBalanceRequest,
			// NetworkName and ChainID will be filled by the service layer if needed,
			// or can be taken from c.netDef here. For now, keep it simple.
		}

		switch reqItem.Type {
		case entity.NativeBalanceRequest:
			batchElems[i] = rpc.BatchElem{
				Method: "eth_getBalance",
				Args:   []interface{}{common.HexToAddress(reqItem.WalletAddress), "latest"},
				Result: new(*hexutil.Big), // eth_getBalance returns hexutil.Big
			}
		case entity.TokenBalanceRequest:
			paddedWalletAddress := common.LeftPadBytes(common.HexToAddress(reqItem.WalletAddress).Bytes(), 32)
			callData := append(erc20MethodID, paddedWalletAddress...)

			// Arguments for eth_call: [{to: tokenAddress, data: callData}, "latest"]
			callArgs := map[string]interface{}{
				"to":   common.HexToAddress(reqItem.TokenAddress),
				"data": hexutil.Bytes(callData),
			}
			batchElems[i] = rpc.BatchElem{
				Method: "eth_call",
				Args:   []interface{}{callArgs, "latest"},
				Result: new(hexutil.Bytes),
			}
		default:
			results[i].Error = fmt.Errorf("unknown balance request type: %v for %s", reqItem.Type, reqItem.TokenSymbol)
		}
	}

	rawRPCClient := c.ethClient.Client()

	// Define a timeout for the RPC batch call itself
	// This is in addition to the connection timeout handled during NewEVMClient
	rpcCallCtx, cancel := context.WithTimeout(ctx, c.rpcCallTimeout)
	defer cancel()

	if err := rawRPCClient.BatchCallContext(rpcCallCtx, batchElems); err != nil {
		// This is a high-level error for the entire batch (e.g., network issue, server error)
		// Individual errors are in batchElems[i].Error
		// We will still process individual errors below, but this indicates a wider problem.
		// For now, let's return this error, assuming it's critical if the whole batch fails.
		// Alternatively, one could iterate through batchElems and populate individual errors.
		return results, fmt.Errorf("RPC batch call failed: %w", err)
	}

	for i, elem := range batchElems {
		if results[i].Error != nil { // Error already set due to unknown type
			continue
		}
		if elem.Error != nil {
			results[i].Error = fmt.Errorf("failed to fetch %s for %s (wallet %s): %w",
				requests[i].TokenSymbol, requests[i].TokenAddress, requests[i].WalletAddress, elem.Error)
			continue
		}

		switch requests[i].Type {
		case entity.NativeBalanceRequest:
			if result, ok := elem.Result.(**hexutil.Big); ok && result != nil && *result != nil {
				results[i].Balance = (*big.Int)(*result)
			} else {
				results[i].Error = fmt.Errorf("failed to decode native balance for %s: unexpected type or nil result", requests[i].TokenSymbol)
			}
		case entity.TokenBalanceRequest:
			if result, ok := elem.Result.(*hexutil.Bytes); ok && result != nil {
				if len(*result) == 0 { // Handle empty result from eth_call as zero balance or error
					// Some nodes might return "0x" for zero balance or if contract doesn't exist/reverts.
					// Others might error. For simplicity, we can treat empty non-error result as 0.
					results[i].Balance = big.NewInt(0)
				} else {
					var balanceVal *big.Int
					// The result of eth_call for balanceOf is the raw bytes of the uint256.
					// We need to convert these bytes to *big.Int.
					// abi.Unpack works on packed arguments, for a single return value we can parse directly.
					// Or ensure `balanceOf` output in ABI is correctly handled by Unpack.
					// Let's try Unpack with the method name.
					unpacked, err := parsedERC20ABI.Unpack("balanceOf", *result)
					if err != nil {
						results[i].Error = fmt.Errorf("failed to unpack balanceOf result for %s: %w. Raw: %s", requests[i].TokenSymbol, err, hexutil.Encode(*result))
						continue
					}
					if len(unpacked) == 0 {
						results[i].Error = fmt.Errorf("balanceOf unpack returned no data for %s", requests[i].TokenSymbol)
						continue
					}
					balanceVal, ok = unpacked[0].(*big.Int)
					if !ok {
						results[i].Error = fmt.Errorf("failed to assert unpacked balanceOf result to *big.Int for %s. Got: %T", requests[i].TokenSymbol, unpacked[0])
						continue
					}
					results[i].Balance = balanceVal
				}
			} else {
				results[i].Error = fmt.Errorf("failed to decode token balance for %s: unexpected type or nil result", requests[i].TokenSymbol)
			}
		}

		if results[i].Error == nil && results[i].Balance != nil {
			formatted, err := utils.FormatBigInt(results[i].Balance, results[i].Decimals)
			if err != nil {
				results[i].Error = fmt.Errorf("failed to format balance for %s: %w", requests[i].TokenSymbol, err)
			} else {
				results[i].FormattedBalance = formatted
			}
		} else if results[i].Error == nil && results[i].Balance == nil {
			// This case might happen if a zero balance was returned as nil or empty bytes and successfully parsed as such.
			// Ensure zero balance is represented correctly.
			results[i].Balance = big.NewInt(0)
			results[i].FormattedBalance = "0"
		}
	}
	return results, nil
}

// Definition returns the network definition for this client.
func (c *EVMClient) Definition() entity.NetworkDefinition {
	return c.netDef
}

// Helper for ERC20 (can be moved to a contracts package later)
// This would be generated by abigen

// MinimalERC20 is a minimal interface for an ERC20 token contract.
// type MinimalERC20 struct {
// 	contract *bind.BoundContract
// }

// func NewMinimalERC20(address common.Address, backend bind.ContractBackend) (*MinimalERC20, error) {
// 	abi, err := abi.JSON(strings.NewReader(erc20ABI))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &MinimalERC20{contract: bind.NewBoundContract(address, abi, backend, backend, backend)}, nil
// }

// func (c *MinimalERC20) BalanceOf(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
// 	var out []interface{}
// 	// Note: in the results slice, the elements are pointers to the actual types.
// 	// So for uint256, it will be *big.Int.
// 	err := c.contract.Call(opts, &out, "balanceOf", account)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// Ensure out is not empty and the first element is of the expected type.
// 	if len(out) == 0 {
// 	    return nil, fmt.Errorf("balanceOf call returned no output")
// 	}
// 	val, ok := out[0].(*big.Int)
// 	if !ok {
// 		return nil, fmt.Errorf("unexpected type for balanceOf result: %T", out[0])
// 	}
// 	return val, nil
// }
