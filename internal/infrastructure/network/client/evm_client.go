package client

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"balance_checker/internal/app/port"
	"balance_checker/internal/domain/entity"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EVMClient implements the port.BlockchainClient interface for EVM-compatible chains.

// EVMClient implements the port.BlockchainClient interface for EVM-compatible chains.
type EVMClient struct {
	ethClient *ethclient.Client
	netDef    entity.NetworkDefinition
	// logger    port.Logger // Optional: if specific logging per client is needed
}

// NewEVMClient creates a new EVM client for the given network definition.
// It tries to connect to the primary RPC URL, then fallbacks if necessary.
func NewEVMClient(netDef entity.NetworkDefinition, httpClient *http.Client, connectionTimeout time.Duration) (port.BlockchainClient, error) {
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
			return &EVMClient{ethClient: client, netDef: netDef}, nil
		}
		lastErr = fmt.Errorf("failed to connect to RPC %s: %w", rpcURL, err)
		// logger.L().Warn("Failed to connect to RPC, trying next", "url", rpcURL, "error", err)
	}

	return nil, fmt.Errorf("all RPC connection attempts failed for network %s: %w", netDef.Name, lastErr)
}

// GetNativeBalance fetches the native currency balance (e.g., ETH, BNB) for a wallet.
func (c *EVMClient) GetNativeBalance(ctx context.Context, walletAddress string) (*big.Int, error) {
	address := common.HexToAddress(walletAddress)
	balance, err := c.ethClient.BalanceAt(ctx, address, nil) // nil for latest block
	if err != nil {
		return nil, fmt.Errorf("failed to get native balance for %s on %s: %w", walletAddress, c.netDef.Name, err)
	}
	return balance, nil
}

// ERC20 ABI minimal part for balanceOf
const erc20ABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}, {"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`

// GetTokenBalance fetches the balance of an ERC20 token for a wallet.
// This is a simplified version. For full ERC20 interaction, use abigen.
func (c *EVMClient) GetTokenBalance(ctx context.Context, tokenAddressHex string, walletAddressHex string) (*big.Int, error) {
	tokenAddress := common.HexToAddress(tokenAddressHex)
	walletAddress := common.HexToAddress(walletAddressHex)

	// A generic way to call a view function on a contract
	// For `balanceOf(address)(uint256)`
	// Method ID for balanceOf(address) is 0x70a08231
	// Data: methodID + padded walletAddress
	methodID := []byte{0x70, 0xa0, 0x82, 0x31} // Method ID of balanceOf(address)
	paddedAddress := common.LeftPadBytes(walletAddress.Bytes(), 32)
	var calldata []byte
	calldata = append(calldata, methodID...)
	calldata = append(calldata, paddedAddress...)

	// Using bind.BoundContract for a slightly more structured call than raw CallContract
	// This requires a more complete ABI definition or a pre-generated binding.
	// For simplicity and directness, let's use a more direct CallContract approach if not using abigen.
	// However, for a real project, using abigen to generate token bindings is highly recommended.

	// Let's use an instance of a generic ERC20 contract structure for balanceOf
	// This would typically be generated by abigen from an ERC20 ABI.
	// For now, we will simulate this by manually crafting the call or using a minimal binding.

	// We need a generic ERC20 contract instance. This is where abigen helps.
	// A simplified example for `balanceOf` might look like this if we had a helper:
	// tokenContract, err := erc20.NewErc20(tokenAddress, c.ethClient) // Assuming erc20 is your abigen generated package
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to instantiate ERC20 contract %s: %w", tokenAddressHex, err)
	// }
	// balance, err := tokenContract.BalanceOf(&bind.CallOpts{Context: ctx}, walletAddress)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to get token balance for %s (token %s) on %s: %w", walletAddressHex, tokenAddressHex, c.netDef.Name, err)
	// }
	// return balance, nil

	// Fallback to a more manual CallContract if abigen isn't set up yet for this step.
	// This is more complex to get right for all cases and return types.
	// A better approach without full abigen is to use a helper that parses a minimal ABI string.

	// For demonstration, let's assume we're using a simplified caller or a pre-generated one later.
	// This part will be placeholder for now and needs proper ERC20 interaction code.
	// The recommended way is to use `abigen` to generate Go bindings for the ERC20 contract.
	// Then you can call: instance, err := NewYourTokenContract(tokenAddress, c.ethClient)
	// balance, err := instance.BalanceOf(nil, walletAddress)

	// Using the generic contract call pattern if abigen is not used.
	// This is more low-level and error-prone than abigen.
	// We will use `bind.NewBoundContract` with a parsed ABI snippet.

	parsedAbi, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 ABI: %w", err)
	}
	contract := bind.NewBoundContract(tokenAddress, parsedAbi, c.ethClient, c.ethClient, c.ethClient)

	var out []interface{}
	err = contract.Call(&bind.CallOpts{Context: ctx}, &out, "balanceOf", walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to call balanceOf for token %s on wallet %s network %s: %w", tokenAddressHex, walletAddressHex, c.netDef.Name, err)
	}

	balance, ok := out[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to assert type of balanceOf output to *big.Int for token %s", tokenAddressHex)
	}
	return balance, nil
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
