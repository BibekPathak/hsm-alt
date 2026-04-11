package ethereum

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// ERC-20 Transfer function selector (keccak256("transfer(address,uint256)")[:4])
var erc20TransferSelector = []byte{0xa9, 0x05, 0x9c, 0xbb}

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

// encodeERC20Transfer encodes the transfer(address,uint256) function call
func encodeERC20Transfer(to string, amount *big.Int) []byte {
	data := make([]byte, 4+32+32)
	copy(data[0:4], erc20TransferSelector)

	// Pad recipient address (bytes 4-36)
	addrBytes := common.HexToAddress(to).Bytes()
	copy(data[4:36], addrBytes)

	// Pad amount (bytes 36-68)
	amountBytes := common.LeftPadBytes(amount.Bytes(), 32)
	copy(data[36:68], amountBytes)

	return data
}

// BuildAndSignERC20TransferTx builds and signs an ERC-20 token transfer
func (b *TxBuilder) BuildAndSignERC20TransferTx(ctx context.Context, tokenContract, to string, amount *big.Int, privateKey *ecdsa.PrivateKey, speed FeeSpeed) (string, string, error) {
	from := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	nonce, err := b.rpcClient.GetNonce(ctx, from)
	if err != nil {
		return "", "", fmt.Errorf("failed to get nonce: %w", err)
	}

	feeInfo, err := b.rpcClient.GetFeeInfo(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get fee info: %w", err)
	}

	var maxFeePerGas, maxPriorityFeePerGas *big.Int
	switch speed {
	case FeeSpeedSlow:
		maxFeePerGas = feeInfo.Slow
	case FeeSpeedFast:
		maxFeePerGas = feeInfo.Fast
		maxPriorityFeePerGas = feeInfo.MaxPriorityFee
		if maxPriorityFeePerGas == nil {
			maxPriorityFeePerGas = big.NewInt(3e9)
		}
	default:
		maxFeePerGas = feeInfo.Standard
		maxPriorityFeePerGas = feeInfo.MaxPriorityFee
		if maxPriorityFeePerGas == nil {
			maxPriorityFeePerGas = big.NewInt(2e9)
		}
	}

	if maxFeePerGas.Cmp(maxPriorityFeePerGas) < 0 {
		maxFeePerGas = maxPriorityFeePerGas
	}

	// Encode ERC-20 transfer data
	data := encodeERC20Transfer(to, amount)
	gasLimit := uint64(65000) // ERC-20 transfer typically uses ~65k gas

	// Build transaction to token contract with data
	toAddr := common.HexToAddress(tokenContract)
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		To:        &toAddr,
		Value:     big.NewInt(0), // No ETH sent, just token
		Gas:       gasLimit,
		GasFeeCap: maxFeePerGas,
		GasTipCap: maxPriorityFeePerGas,
		Data:      data,
	})

	chainIDBig := big.NewInt(b.chainID)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainIDBig), privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign tx: %w", err)
	}

	txBytes, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	rawTxHex := fmt.Sprintf("0x%x", txBytes)

	return txHash, rawTxHex, nil
}

// GetTokenBalance gets the ERC-20 token balance for an address
func (b *TxBuilder) GetTokenBalance(ctx context.Context, tokenContract, address string) (*big.Int, error) {
	// Encode balanceOf(address) call
	// Function selector: 0x70a08231
	addrBytes := common.HexToAddress(address).Bytes()
	data := make([]byte, 36)
	copy(data[4:], common.LeftPadBytes(addrBytes, 32))

	// Use eth_call to query the contract
	result, err := b.rpcClient.Call(ctx, map[string]interface{}{
		"to":   tokenContract,
		"data": "0x70a08231" + hex.EncodeToString(common.LeftPadBytes(addrBytes, 32)),
	}, "latest")
	if err != nil {
		return nil, fmt.Errorf("failed to call token contract: %w", err)
	}

	if result == "" || result == "0x" {
		return big.NewInt(0), nil
	}

	return common.HexToHash(result).Big(), nil
}

// CheckERC20BalanceSufficient checks if account has enough token balance
func (b *TxBuilder) CheckERC20BalanceSufficient(ctx context.Context, tokenContract, address string, tokenAmount *big.Int, gasLimit uint64) (bool, *big.Int, *big.Int, error) {
	tokenBalance, err := b.GetTokenBalance(ctx, tokenContract, address)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to get token balance: %w", err)
	}

	sufficient := tokenBalance.Cmp(tokenAmount) >= 0
	if !sufficient {
		return false, tokenBalance, nil, nil
	}

	// Also check ETH balance for gas
	ethBalance, err := b.rpcClient.GetBalance(ctx, address)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to get ETH balance: %w", err)
	}

	feeInfo, err := b.rpcClient.GetFeeInfo(ctx)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to get fee info: %w", err)
	}

	gasFee := new(big.Int).Mul(big.NewInt(int64(gasLimit)), feeInfo.Standard)
	sufficientEth := ethBalance.Cmp(gasFee) >= 0

	return sufficientEth, tokenBalance, gasFee, nil
}

// EstimateERC20Gas estimates gas for an ERC-20 transfer
func (b *TxBuilder) EstimateERC20Gas(ctx context.Context, from, tokenContract, to string, amount *big.Int) (uint64, error) {
	data := encodeERC20Transfer(to, amount)

	msg := map[string]interface{}{
		"from":  from,
		"to":    tokenContract,
		"value": "0x0",
		"data":  "0x" + hex.EncodeToString(data),
	}

	result, err := b.rpcClient.call(ctx, "eth_estimateGas", msg)
	if err != nil {
		return 65000, nil // Default ERC-20 gas
	}

	var gasHex string
	if err := json.Unmarshal(result, &gasHex); err != nil {
		return 65000, nil
	}

	return parseHexUint64(gasHex), nil
}
