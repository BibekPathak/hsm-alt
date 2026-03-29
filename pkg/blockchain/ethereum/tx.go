package ethereum

import (
	"context"
	"crypto/ecdsa"
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
	if err := result.UnmarshalJSON([]byte(gasHex)); err != nil {
		return 21000, nil // Default gas limit for ETH transfer
	}

	return parseHexUint64(gasHex), nil
}
