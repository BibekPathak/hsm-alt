package solana

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/mr-tron/base58"
)

const (
	SystemProgramID = "11111111111111111111111111111111"
	// SPL Token Program IDs
	TokenProgramID     = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	AssociatedTokenPID = "ATokenGPvbdGVxr1b2hvZbcYZgXXGMz7GaWNjYdNW"
)

type TxBuilder struct {
	rpcClient *RPCClient
}

func NewTxBuilder(rpcClient *RPCClient) *TxBuilder {
	return &TxBuilder{
		rpcClient: rpcClient,
	}
}

type MessageHeader struct {
	NumRequiredSignatures       uint8
	NumReadonlySignedAccounts   uint8
	NumReadonlyUnsignedAccounts uint8
}

type CompiledInstruction struct {
	ProgramIDIndex uint8
	Accounts       []uint8
	Data           []byte
}

type Message struct {
	Header          MessageHeader
	AccountKeys     []string
	RecentBlockhash string
	Instructions    []CompiledInstruction
}

type Transaction struct {
	Signatures [][]byte
	Message    Message
}

func (b *TxBuilder) BuildTransferTx(ctx context.Context, from, to string, lamports uint64) ([]byte, error) {
	blockhash, err := b.rpcClient.GetLatestBlockhash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blockhash: %w", err)
	}

	msg, err := b.createTransferMessage(from, to, lamports, blockhash.Blockhash)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return b.serializeMessage(msg)
}

func (b *TxBuilder) createTransferMessage(from, to string, lamports uint64, recentBlockhash string) (*Message, error) {
	fromIdx, toIdx := uint8(0), uint8(1)
	programIdx := uint8(2)

	msg := &Message{
		Header: MessageHeader{
			NumRequiredSignatures:       1,
			NumReadonlySignedAccounts:   0,
			NumReadonlyUnsignedAccounts: 1,
		},
		AccountKeys:     []string{from, to, SystemProgramID},
		RecentBlockhash: recentBlockhash,
		Instructions: []CompiledInstruction{
			{
				ProgramIDIndex: programIdx,
				Accounts:       []uint8{fromIdx, toIdx},
				Data:           b.createTransferInstructionData(lamports),
			},
		},
	}

	return msg, nil
}

func (b *TxBuilder) createTransferInstructionData(lamports uint64) []byte {
	// System program transfer instruction: [instruction_index(4 bytes)][lamports(8 bytes)]
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], 2)         // Transfer instruction index
	binary.LittleEndian.PutUint64(data[4:12], lamports) // Amount in lamports
	return data
}

// serializeMessage creates the unsigned message that needs to be signed
func (b *TxBuilder) serializeMessage(msg *Message) ([]byte, error) {
	var buf bytes.Buffer

	// Message header
	buf.WriteByte(byte(msg.Header.NumRequiredSignatures))
	buf.WriteByte(msg.Header.NumReadonlySignedAccounts)
	buf.WriteByte(msg.Header.NumReadonlyUnsignedAccounts)

	// Account keys (compact array format)
	if err := b.writeCompactArray(&buf, len(msg.AccountKeys)); err != nil {
		return nil, fmt.Errorf("failed to write account keys length: %w", err)
	}
	for _, key := range msg.AccountKeys {
		decoded, err := base58.Decode(key)
		if err != nil {
			return nil, fmt.Errorf("failed to decode account key: %w", err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("account key must be 32 bytes, got %d", len(decoded))
		}
		buf.Write(decoded)
	}

	// Recent blockhash
	blockhash, err := base58.Decode(msg.RecentBlockhash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode blockhash: %w", err)
	}
	if len(blockhash) != 32 {
		return nil, fmt.Errorf("blockhash must be 32 bytes, got %d", len(blockhash))
	}
	buf.Write(blockhash)

	// Instructions (compact array format)
	if err := b.writeCompactArray(&buf, len(msg.Instructions)); err != nil {
		return nil, fmt.Errorf("failed to write instructions length: %w", err)
	}

	for _, inst := range msg.Instructions {
		buf.WriteByte(inst.ProgramIDIndex)

		// Accounts (compact array format)
		if err := b.writeCompactArray(&buf, len(inst.Accounts)); err != nil {
			return nil, fmt.Errorf("failed to write instruction accounts length: %w", err)
		}
		buf.Write(inst.Accounts)

		// Data (compact array format)
		if err := b.writeCompactArray(&buf, len(inst.Data)); err != nil {
			return nil, fmt.Errorf("failed to write instruction data length: %w", err)
		}
		buf.Write(inst.Data)
	}

	return buf.Bytes(), nil
}

// writeCompactArray writes length in Solana's compact-u16 format
func (b *TxBuilder) writeCompactArray(buf *bytes.Buffer, length int) error {
	if length < 0 {
		return fmt.Errorf("length cannot be negative: %d", length)
	}

	if length <= 127 {
		buf.WriteByte(byte(length))
	} else if length <= 16383 {
		buf.WriteByte(byte(length) | 0x80)
		buf.WriteByte(byte(length >> 7))
	} else if length <= 2097151 {
		buf.WriteByte(byte(length) | 0x80)
		buf.WriteByte(byte(length>>7) | 0x80)
		buf.WriteByte(byte(length >> 14))
	} else {
		return fmt.Errorf("compact array length too large: %d", length)
	}

	return nil
}

func (b *TxBuilder) AddSignature(unsignedTx []byte, signature []byte) ([]byte, error) {
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	// Solana wire format: [compact_array_len][signature1...signatureN][message]
	var buf bytes.Buffer

	// Write number of signatures using compact array format
	if err := b.writeCompactArray(&buf, 1); err != nil {
		return nil, fmt.Errorf("failed to write signature count: %w", err)
	}

	// Write the signature (exactly 64 bytes)
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be exactly 64 bytes, got %d", len(signature))
	}
	buf.Write(signature)

	// Write the unsigned message
	buf.Write(unsignedTx)

	return buf.Bytes(), nil
}

func (b *TxBuilder) SerializeWithSignatures(msg *Message, signatures [][]byte) ([]byte, error) {
	var buf bytes.Buffer

	// Write signature count using compact array format
	if err := b.writeCompactArray(&buf, len(signatures)); err != nil {
		return nil, fmt.Errorf("failed to write signature count: %w", err)
	}

	// Write all signatures (each must be exactly 64 bytes)
	for i, sig := range signatures {
		if len(sig) != 64 {
			return nil, fmt.Errorf("signature %d must be exactly 64 bytes, got %d", i, len(sig))
		}
		buf.Write(sig)
	}

	// Serialize the message
	msgBytes, err := b.serializeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}
	buf.Write(msgBytes)

	return buf.Bytes(), nil
}

func (b *TxBuilder) SendTransaction(ctx context.Context, signedTx []byte) (string, error) {
	return b.rpcClient.SendTransaction(ctx, signedTx)
}

func (b *TxBuilder) GetBalance(ctx context.Context, address string) (uint64, error) {
	return b.rpcClient.GetBalance(ctx, address)
}

func (b *TxBuilder) CheckBalanceSufficient(ctx context.Context, address string, lamports uint64) (bool, uint64, error) {
	balance, err := b.rpcClient.GetBalance(ctx, address)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get balance: %w", err)
	}

	sufficient := balance >= lamports
	return sufficient, lamports, nil
}

func (b *TxBuilder) GetFeeEstimate(ctx context.Context) (uint64, error) {
	_, err := b.rpcClient.GetLatestBlockhash(ctx)
	if err != nil {
		return 5000, nil
	}

	recentPrioritization, err := b.rpcClient.call(ctx, "getRecentPrioritizationFees", SystemProgramID)
	if err != nil {
		return 5000, nil
	}

	var fees []struct {
		PrioritizationFee uint64 `json:"prioritizationFee"`
	}
	if err := json.Unmarshal(recentPrioritization, &fees); err != nil || len(fees) == 0 {
		return 5000, nil
	}

	computeUnits := uint64(200)
	priorityFee := fees[0].PrioritizationFee
	fee := computeUnits*1000 + priorityFee

	return fee, nil
}

func (b *TxBuilder) ConfirmTransaction(ctx context.Context, signature string) error {
	return b.rpcClient.WaitForConfirmation(ctx, signature)
}

func (b *TxBuilder) GetBlockhash(ctx context.Context) (string, error) {
	blockhash, err := b.rpcClient.GetLatestBlockhash(ctx)
	if err != nil {
		return "", err
	}
	return blockhash.Blockhash, nil
}

func (b *TxBuilder) BuildTransferTxWithBlockhash(ctx context.Context, from, to string, lamports uint64, blockhash string) ([]byte, error) {
	msg, err := b.createTransferMessage(from, to, lamports, blockhash)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return b.serializeMessage(msg)
}

// GetAssociatedTokenAddress derives the ATA for a wallet and token mint
func GetAssociatedTokenAddress(owner, mint string) (string, error) {
	ownerPubkey, err := base58.Decode(owner)
	if err != nil {
		return "", fmt.Errorf("failed to decode owner: %w", err)
	}

	mintPubkey, err := base58.Decode(mint)
	if err != nil {
		return "", fmt.Errorf("failed to decode mint: %w", err)
	}

	programID, _ := base58.Decode(AssociatedTokenPID)

	// Simple hash for ATA derivation: owner || mint || program
	ataData := append(ownerPubkey[:32], mintPubkey[:32]...)
	ataData = append(ataData, programID[:32]...)

	// Use hash as deterministic seed
	hasher := sha256.New()
	hasher.Write(ataData)
	hash := hasher.Sum(nil)

	// Take first 32 bytes and encode as address
	ataBytes := append([]byte{0x03}, hash[:31]...) // Version byte + 31 hash bytes

	return base58.Encode(ataBytes), nil
}

// GetTokenAccountBalance gets the SPL token balance for an ATA
func (b *TxBuilder) GetTokenAccountBalance(ctx context.Context, ata string) (uint64, error) {
	result, err := b.rpcClient.call(ctx, "getTokenAccountBalance", ata)
	if err != nil {
		return 0, fmt.Errorf("failed to get token balance: %w", err)
	}

	var response struct {
		Value struct {
			Amount   string `json:"amount"`
			Decimals uint8  `json:"decimals"`
		} `json:"value"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	var amount uint64
	fmt.Sscanf(response.Value.Amount, "%d", &amount)
	return amount, nil
}

// GetTokenBalance gets token balance for an owner (finds ATA first)
func (b *TxBuilder) GetTokenBalance(ctx context.Context, owner, mint string) (uint64, error) {
	ata, err := GetAssociatedTokenAddress(owner, mint)
	if err != nil {
		return 0, err
	}

	// Check if ATA exists
	exists, err := b.rpcClient.call(ctx, "getAccountInfo", ata, map[string]interface{}{"encoding": "base64"})
	if err != nil {
		return 0, nil // Account doesn't exist = 0 balance
	}

	var response struct {
		Value interface{} `json:"value"`
	}
	if err := json.Unmarshal(exists, &response); err != nil {
		return 0, nil
	}

	if response.Value == nil {
		return 0, nil // ATA doesn't exist
	}

	return b.GetTokenAccountBalance(ctx, ata)
}

// createSPLTransferInstruction creates a SPL token transfer instruction
func (b *TxBuilder) createSPLTransferInstruction(source, dest, mint, authority string, amount uint64) CompiledInstruction {
	// SPL Transfer instruction: transfer(checker, source, dest, authority, signers[], amount)
	// Program data: [transfer instruction (1 byte) + amount (8 bytes)]
	data := make([]byte, 9)
	data[0] = 3 // Transfer instruction index

	// Encode amount as little-endian 64-bit
	binary.LittleEndian.PutUint64(data[1:], amount)

	return CompiledInstruction{
		ProgramIDIndex: 2,                      // Token program
		Accounts:       []uint8{4, 5, 1, 0, 3}, // source, dest, mint, authority, (signer)
		Data:           data,
	}
}

// BuildSPLTransferTx builds an SPL token transfer transaction
func (b *TxBuilder) BuildSPLTransferTx(ctx context.Context, from, to, mint string, amount uint64) ([]byte, error) {
	blockhash, err := b.rpcClient.GetLatestBlockhash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Derive ATAs (simplified - real implementation uses proper PDA derivation)
	fromAta := from // In real impl: derive ATA(from, mint)
	toAta := to     // In real impl: derive ATA(to, mint)

	// For now, use simplified message structure - just token program accounts
	msg := &Message{
		Header: MessageHeader{
			NumRequiredSignatures:       1,
			NumReadonlySignedAccounts:   2,
			NumReadonlyUnsignedAccounts: 2,
		},
		AccountKeys: []string{
			from,            // 0: fee payer + source authority (signer)
			fromAta,         // 1: source ATA
			toAta,           // 2: destination ATA
			mint,            // 3: token mint
			from,            // 4: authority (signer)
			TokenProgramID,  // 5: token program
			SystemProgramID, // 6: system program (for any needed transfers)
		},
		RecentBlockhash: blockhash.Blockhash,
		Instructions: []CompiledInstruction{
			{
				ProgramIDIndex: 5,                      // Token program
				Accounts:       []uint8{1, 2, 3, 0, 0}, // source, dest, mint, authority (all same for now)
				Data:           b.createSPLTransferData(amount),
			},
		},
	}

	return b.serializeMessage(msg)
}

func (b *TxBuilder) createSPLTransferData(amount uint64) []byte {
	data := make([]byte, 9)
	data[0] = 3 // Transfer instruction
	binary.LittleEndian.PutUint64(data[1:], amount)
	return data
}

// CheckSPLBalanceSufficient checks if account has enough token balance
func (b *TxBuilder) CheckSPLBalanceSufficient(ctx context.Context, owner, mint string, amount uint64) (bool, uint64, error) {
	balance, err := b.GetTokenBalance(ctx, owner, mint)
	if err != nil {
		return false, 0, err
	}

	sufficient := balance >= amount
	return sufficient, balance, nil
}
