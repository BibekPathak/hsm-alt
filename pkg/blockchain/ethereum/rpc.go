package ethereum

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// RPCClient implements Ethereum JSON-RPC client
type RPCClient struct {
	url        string
	httpClient *http.Client
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TransactionResult represents the result of a transaction submission
type TransactionResult struct {
	TxHash    string
	GasUsed   *big.Int
	BlockNum  uint64
	Confirmed bool
}

// NewRPCClient creates a new Ethereum RPC client
func NewRPCClient(url string) *RPCClient {
	return &RPCClient{
		url: url,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetNonce returns the current nonce for an address
func (c *RPCClient) GetNonce(ctx context.Context, address string) (uint64, error) {
	result, err := c.call(ctx, "eth_getTransactionCount", address, "latest")
	if err != nil {
		return 0, fmt.Errorf("failed to get nonce: %w", err)
	}

	var nonceHex string
	if err := json.Unmarshal(result, &nonceHex); err != nil {
		return 0, fmt.Errorf("failed to parse nonce: %w", err)
	}

	return parseHexUint64(nonceHex), nil
}

// GetBalance returns the ETH balance for an address (in wei)
func (c *RPCClient) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	result, err := c.call(ctx, "eth_getBalance", address, "latest")
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	var balanceHex string
	if err := json.Unmarshal(result, &balanceHex); err != nil {
		return nil, fmt.Errorf("failed to parse balance: %w", err)
	}

	balance := new(big.Int)
	balance.SetString(balanceHex[2:], 16)
	return balance, nil
}

// GetChainID returns the current chain ID
func (c *RPCClient) GetChainID(ctx context.Context) (uint64, error) {
	result, err := c.call(ctx, "eth_chainId")
	if err != nil {
		return 0, fmt.Errorf("failed to get chain ID: %w", err)
	}

	var chainIDHex string
	if err := json.Unmarshal(result, &chainIDHex); err != nil {
		return 0, fmt.Errorf("failed to parse chain ID: %w", err)
	}

	return parseHexUint64(chainIDHex), nil
}

// GetGasPrice returns the current gas price (in wei)
func (c *RPCClient) GetGasPrice(ctx context.Context) (*big.Int, error) {
	result, err := c.call(ctx, "eth_gasPrice")
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	var gasPriceHex string
	if err := json.Unmarshal(result, &gasPriceHex); err != nil {
		return nil, fmt.Errorf("failed to parse gas price: %w", err)
	}

	gasPrice := new(big.Int)
	gasPrice.SetString(gasPriceHex[2:], 16)
	return gasPrice, nil
}

// SendRawTransaction broadcasts a signed transaction
func (c *RPCClient) SendRawTransaction(ctx context.Context, signedTxHex string) (string, error) {
	result, err := c.call(ctx, "eth_sendRawTransaction", signedTxHex)
	if err != nil {
		return "", fmt.Errorf("failed to send raw tx: %w", err)
	}

	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		return "", fmt.Errorf("failed to parse tx hash: %w", err)
	}

	return txHash, nil
}

// GetTransactionReceipt returns the receipt for a transaction
func (c *RPCClient) GetTransactionReceipt(ctx context.Context, txHash string) (*TransactionResult, error) {
	result, err := c.call(ctx, "eth_getTransactionReceipt", txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt: %w", err)
	}

	if result == nil {
		return &TransactionResult{Confirmed: false}, nil
	}

	var receipt struct {
		TxHash      string `json:"transactionHash"`
		GasUsed     string `json:"gasUsed"`
		BlockNumber string `json:"blockNumber"`
	}

	if err := json.Unmarshal(result, &receipt); err != nil {
		return nil, fmt.Errorf("failed to parse receipt: %w", err)
	}

	gasUsed := new(big.Int)
	gasUsed.SetString(receipt.GasUsed[2:], 16)

	return &TransactionResult{
		TxHash:    receipt.TxHash,
		GasUsed:   gasUsed,
		BlockNum:  parseHexUint64(receipt.BlockNumber),
		Confirmed: true,
	}, nil
}

// call executes a JSON-RPC call
func (c *RPCClient) call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, strings.NewReader(string(reqData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// parseHexUint64 parses a hex string (0x...) to uint64
func parseHexUint64(hex string) uint64 {
	if len(hex) <= 2 {
		return 0
	}

	val := new(big.Int)
	val.SetString(hex[2:], 16)
	return val.Uint64()
}
