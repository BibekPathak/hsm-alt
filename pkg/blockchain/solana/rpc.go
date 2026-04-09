package solana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mr-tron/base58"
)

const (
	SolDecimals         = 9
	LamportsPerSOL      = 1e9
	DefaultRPCURL       = "https://api.devnet.solana.com"
	DefaultWSURL        = "wss://api.devnet.solana.com"
	ConfirmationTimeout = 30 * time.Second
)

type RPCClient struct {
	url        string
	httpClient *http.Client
}

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Blockhash struct {
	Blockhash      string `json:"blockhash"`
	LastValidBlock uint64 `json:"lastValidBlockHeight"`
}

type SignatureStatus struct {
	Slot          uint64      `json:"slot"`
	Confirmations uint64      `json:"confirmations,string"`
	Err           interface{} `json:"err"`
	Status        string      `json:"status"`
}

type GetBalanceResult struct {
	Value   uint64 `json:"value"`
	Context struct {
		Slot uint64 `json:"slot"`
	} `json:"context"`
}

type SendTransactionResult struct {
	Context struct {
		Slot uint64 `json:"slot"`
	} `json:"context"`
	Signature string `json:"signature"`
}

type GetSignatureStatusesResult struct {
	Context struct {
		Slot uint64 `json:"slot"`
	} `json:"context"`
	Value []SignatureStatus `json:"value"`
}

func NewRPCClient(url string) *RPCClient {
	if url == "" {
		url = DefaultRPCURL
	}
	return &RPCClient{
		url: url,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *RPCClient) GetBalance(ctx context.Context, address string) (uint64, error) {
	result, err := c.call(ctx, "getBalance", address)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	var balance GetBalanceResult
	if err := json.Unmarshal(result, &balance); err != nil {
		return 0, fmt.Errorf("failed to parse balance: %w", err)
	}

	return balance.Value, nil
}

func (c *RPCClient) GetLatestBlockhash(ctx context.Context) (*Blockhash, error) {
	result, err := c.call(ctx, "getLatestBlockhash")
	if err != nil {
		return nil, fmt.Errorf("failed to get blockhash: %w", err)
	}

	var blockhash Blockhash
	if err := json.Unmarshal(result, &blockhash); err != nil {
		return nil, fmt.Errorf("failed to parse blockhash: %w", err)
	}

	return &blockhash, nil
}

func (c *RPCClient) SendTransaction(ctx context.Context, signedTx []byte) (string, error) {
	txBase64 := base64.StdEncoding.EncodeToString(signedTx)

	result, err := c.call(ctx, "sendTransaction", txBase64)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	var sendResult SendTransactionResult
	if err := json.Unmarshal(result, &sendResult); err != nil {
		return "", fmt.Errorf("failed to parse send result: %w", err)
	}

	return sendResult.Signature, nil
}

func (c *RPCClient) GetSignatureStatus(ctx context.Context, signature string) (*SignatureStatus, error) {
	result, err := c.call(ctx, "getSignatureStatuses", []string{signature})
	if err != nil {
		return nil, fmt.Errorf("failed to get signature status: %w", err)
	}

	var statusResult GetSignatureStatusesResult
	if err := json.Unmarshal(result, &statusResult); err != nil {
		return nil, fmt.Errorf("failed to parse signature status: %w", err)
	}

	if len(statusResult.Value) == 0 {
		return nil, nil
	}

	return &statusResult.Value[0], nil
}

func (c *RPCClient) WaitForConfirmation(ctx context.Context, signature string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(ConfirmationTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("confirmation timeout")
		case <-ticker.C:
			status, err := c.GetSignatureStatus(ctx, signature)
			if err != nil {
				continue
			}
			if status == nil {
				continue
			}
			if status.Err != nil {
				return fmt.Errorf("transaction failed: %v", status.Err)
			}
			if status.Status == "confirmed" || status.Status == "finalized" {
				return nil
			}
		}
	}
}

func (c *RPCClient) GetAccountInfo(ctx context.Context, address string) ([]byte, error) {
	result, err := c.call(ctx, "getAccountInfo", address, map[string]interface{}{
		"encoding": "base64",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	var response struct {
		Value struct {
			Data []string `json:"data"`
		} `json:"value"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse account info: %w", err)
	}

	if len(response.Value.Data) == 0 {
		return nil, nil
	}

	return base64.StdEncoding.DecodeString(response.Value.Data[0])
}

func (c *RPCClient) GetMinimumBalanceForRentExemption(ctx context.Context, dataSize uint64) (uint64, error) {
	result, err := c.call(ctx, "getMinimumBalanceForRentExemption", dataSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get rent exemption: %w", err)
	}

	var lamports uint64
	if err := json.Unmarshal(result, &lamports); err != nil {
		return 0, fmt.Errorf("failed to parse rent: %w", err)
	}

	return lamports, nil
}

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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func SOLToLamports(sol float64) uint64 {
	return uint64(sol * float64(LamportsPerSOL))
}

func LamportsToSOL(lamports uint64) float64 {
	return float64(lamports) / float64(LamportsPerSOL)
}

func ParseSOL(value string) (uint64, error) {
	var sol float64
	_, err := fmt.Sscanf(value, "%f", &sol)
	if err != nil {
		return 0, fmt.Errorf("invalid SOL value: %w", err)
	}
	return SOLToLamports(sol), nil
}

func ValidateAddress(address string) bool {
	_, err := base58.Decode(address)
	return err == nil && len(address) >= 32 && len(address) <= 44
}
