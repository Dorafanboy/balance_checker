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
func NewEVMClient(netDef entity.NetworkDefinition, httpClient *http.Client, connectionTimeout time.Duration, rpcCallTimeout time.Duration) (port.BlockchainClient, error) {
	initParsedERC20ABI()
	rpcURLs := append([]string{netDef.PrimaryRPCURL}, netDef.FallbackRPCURLs...)
	var lastErr error

	for _, rpcURL := range rpcURLs {
		ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)

		client, err := ethclient.DialContext(ctx, rpcURL)
		cancel()

		if err == nil {
			// Optional: Verify connection with a quick call like ChainID, though not strictly necessary here
			// currentChainID, chainErr := client.ChainID(context.Background())
			// if chainErr != nil { lastErr = fmt.Errorf("failed to verify chainID for %s: %w", rpcURL, chainErr); continue }
			// if currentChainID.Uint64() != netDef.ChainID { lastErr = fmt.Errorf("chainID mismatch for %s: expected %d, got %d", rpcURL, netDef.ChainID, currentChainID.Uint64()); continue }
			return &EVMClient{ethClient: client, netDef: netDef, rpcCallTimeout: rpcCallTimeout}, nil
		}
		lastErr = fmt.Errorf("failed to connect to RPC %s: %w", rpcURL, err)
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
		results[i] = entity.BalanceResultItem{
			RequestID:     reqItem.ID,
			WalletAddress: reqItem.WalletAddress,
			TokenAddress:  reqItem.TokenAddress,
			TokenSymbol:   reqItem.TokenSymbol,
			Decimals:      reqItem.TokenDecimals,
			IsNative:      reqItem.Type == entity.NativeBalanceRequest,
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

	rpcCallCtx, cancel := context.WithTimeout(ctx, c.rpcCallTimeout)
	defer cancel()

	if err := rawRPCClient.BatchCallContext(rpcCallCtx, batchElems); err != nil {
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
				if len(*result) == 0 {
					results[i].Balance = big.NewInt(0)
				} else {
					var balanceVal *big.Int
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
