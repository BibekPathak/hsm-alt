package ethereum

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// TxBuilder builds and signs Ethereum transactions
type TxBuilder struct {
	rpcClient *RPCClient
	chainID   int64
}

// NewTxBuilder creates a new transaction builder
func NewTxBuilder(rpcClient *RPCClient, chainID int64) *TxBuilder {
	return &TxBuilder{
		rpcClient: rpcClient,
		chainID:   chainID,
	}
}

// BuildAndSignTx builds, signs, and encodes a transaction
func (b *TxBuilder) BuildAndSignTx(ctx context.Context, to string, value *big.Int, privateKey *ecdsa.PrivateKey) (string, string, error) {
	from := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// Get nonce
	nonce, err := b.rpcClient.GetNonce(ctx, from)
	if err != nil {
		return "", "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := b.rpcClient.GetGasPrice(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Build transaction
	toAddr := common.HexToAddress(to)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &toAddr,
		Value:    value,
		Gas:      21000, // Standard ETH transfer gas
		GasPrice: gasPrice,
	})

	// Sign transaction
	chainIDBig := big.NewInt(b.chainID)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainIDBig), privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign tx: %w", err)
	}

	// Encode transaction
	txBytes, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	rawTxHex := fmt.Sprintf("0x%x", txBytes)

	return txHash, rawTxHex, nil
}

// BroadcastTransaction broadcasts a raw signed transaction
func (b *TxBuilder) BroadcastTransaction(ctx context.Context, rawTxHex string) (string, error) {
	txHash, err := b.rpcClient.SendRawTransaction(ctx, rawTxHex)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast: %w", err)
	}

	return txHash, nil
}

// GetBalance gets the balance for an address
func (b *TxBuilder) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	return b.rpcClient.GetBalance(ctx, address)
}

// EstimateGas estimates gas for a transaction
func (b *TxBuilder) EstimateGas(ctx context.Context, from, to string, value *big.Int) (uint64, error) {
	msg := map[string]interface{}{
		"from":  from,
		"to":    to,
		"value": fmt.Sprintf("0x%x", value),
	}

	result, err := b.rpcClient.call(ctx, "eth_estimateGas", msg)
	if err != nil {
		return 0, err
	}

	var gasHex string
	if err := json.Unmarshal(result, &gasHex); err != nil {
		return 21000, nil // Default gas limit for ETH transfer
	}

	return parseHexUint64(gasHex), nil
}

// FeeSpeed represents the fee speed preset
type FeeSpeed string

const (
	FeeSpeedSlow     FeeSpeed = "slow"
	FeeSpeedStandard FeeSpeed = "standard"
	FeeSpeedFast     FeeSpeed = "fast"
)

// BuildAndSignTxEIP1559 builds and signs an EIP-1559 transaction
func (b *TxBuilder) BuildAndSignTxEIP1559(ctx context.Context, to string, value *big.Int, privateKey *ecdsa.PrivateKey, speed FeeSpeed) (string, string, error) {
	from := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// Get nonce
	nonce, err := b.rpcClient.GetNonce(ctx, from)
	if err != nil {
		return "", "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get fee info
	feeInfo, err := b.rpcClient.GetFeeInfo(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get fee info: %w", err)
	}

	// Select max fee based on speed
	var maxFeePerGas *big.Int
	switch speed {
	case FeeSpeedSlow:
		maxFeePerGas = feeInfo.Slow
	case FeeSpeedFast:
		maxFeePerGas = feeInfo.Fast
	default:
		maxFeePerGas = feeInfo.Standard
	}

	// Max priority fee (tip to validator)
	maxPriorityFeePerGas := feeInfo.MaxPriorityFee
	if maxPriorityFeePerGas == nil {
		maxPriorityFeePerGas = big.NewInt(2e9) // 2 gwei default
	}

	// Ensure max fee >= priority fee
	if maxFeePerGas.Cmp(maxPriorityFeePerGas) < 0 {
		maxFeePerGas = maxPriorityFeePerGas
	}

	// Build EIP-1559 transaction
	toAddr := common.HexToAddress(to)
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		To:        &toAddr,
		Value:     value,
		Gas:       21000,
		GasFeeCap: maxFeePerGas,
		GasTipCap: maxPriorityFeePerGas,
	})

	// Sign transaction
	chainIDBig := big.NewInt(b.chainID)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainIDBig), privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign tx: %w", err)
	}

	// Encode transaction
	txBytes, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	rawTxHex := fmt.Sprintf("0x%x", txBytes)

	return txHash, rawTxHex, nil
}

// GetFeeInfo returns current fee information
func (b *TxBuilder) GetFeeInfo(ctx context.Context) (*FeeInfo, error) {
	return b.rpcClient.GetFeeInfo(ctx)
}

// CheckBalanceSufficient checks if account has enough balance for value + gas
func (b *TxBuilder) CheckBalanceSufficient(ctx context.Context, address string, value *big.Int, gasLimit uint64) (bool, *big.Int, error) {
	balance, err := b.rpcClient.GetBalance(ctx, address)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get fee info for estimation
	feeInfo, err := b.rpcClient.GetFeeInfo(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get fee info: %w", err)
	}

	// Estimate max fee: gasLimit * maxFeePerGas (use standard)
	gasFee := new(big.Int).Mul(big.NewInt(int64(gasLimit)), feeInfo.Standard)

	// Required = value + gas
	required := new(big.Int).Add(value, gasFee)

	sufficient := balance.Cmp(required) >= 0
	return sufficient, required, nil
}
