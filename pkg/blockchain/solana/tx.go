package solana

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"

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
		lower7Bits := byte(length & 0x7f)
		buf.WriteByte(lower7Bits | 0x80)
		buf.WriteByte(byte(length >> 7))
	} else if length <= 2097151 {
		lower7Bits := byte(length & 0x7f)
		next7Bits := byte((length >> 7) & 0x7f)
		buf.WriteByte(lower7Bits | 0x80)
		buf.WriteByte(next7Bits | 0x80)
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

func (b *TxBuilder) SendTransaction(ctx context.Context, signedTx []byte) (string, error) {
	return b.rpcClient.SendTransaction(ctx, signedTx)
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
// NOTE: This is a placeholder. In production, you should use the proper PDA derivation
// or query the RPC for the actual token accounts
func GetAssociatedTokenAddress(owner, mint string) (string, error) {
	if owner == "" {
		return "", fmt.Errorf("owner address cannot be empty")
	}
	if mint == "" {
		return "", fmt.Errorf("mint address cannot be empty")
	}

	// For now, return empty string to force lookup via RPC
	// The actual ATA derivation requires complex Ed25519 curve checks
	// that are better handled by the Solana SDK
	log.Printf("[ATA] Skipping local derivation, will use RPC to find token accounts")
	return "", nil
}

// GetTokenAccountBalance gets the SPL token balance for an ATA
func (b *TxBuilder) GetTokenAccountBalance(ctx context.Context, ata string) (uint64, error) {
	log.Printf("[SPL] GetTokenAccountBalance: calling getTokenAccountBalance for ATA=%s", ata)

	result, err := b.rpcClient.call(ctx, "getTokenAccountBalance", ata)
	if err != nil {
		log.Printf("[SPL] GetTokenAccountBalance ERROR: RPC call failed: %v", err)
		return 0, fmt.Errorf("failed to get token balance: %w", err)
	}

	log.Printf("[SPL] GetTokenAccountBalance: response=%s", string(result))

	var response struct {
		Value struct {
			Amount   string `json:"amount"`
			Decimals uint8  `json:"decimals"`
		} `json:"value"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		log.Printf("[SPL] GetTokenAccountBalance ERROR: parse failed: %v", err)
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	var amount uint64
	fmt.Sscanf(response.Value.Amount, "%d", &amount)
	log.Printf("[SPL] GetTokenAccountBalance: parsed amount=%d (raw)", amount)
	return amount, nil
}

// GetTokenBalance gets token balance for an owner (finds ATA first)
func (b *TxBuilder) GetTokenBalance(ctx context.Context, owner, mint string) (uint64, error) {
	log.Printf("[SPL] GetTokenBalance: querying all token accounts for owner=%s, mint=%s", owner, mint)

	ata, err := GetAssociatedTokenAddress(owner, mint)
	if err != nil {
		log.Printf("[SPL] GetTokenBalance ERROR: ATA derivation failed: %v", err)
		return 0, err
	}
	log.Printf("[SPL] GetTokenBalance: derived ATA=%s", ata)

	// First try with the derived ATA
	exists, err := b.rpcClient.call(ctx, "getAccountInfo", ata, map[string]interface{}{"encoding": "base64"})
	if err != nil {
		log.Printf("[SPL] GetTokenBalance: error checking derived ATA: %v", err)
	}

	if exists != nil {
		var response struct {
			Value interface{} `json:"value"`
		}
		json.Unmarshal(exists, &response)
		if response.Value != nil {
			log.Printf("[SPL] GetTokenBalance: derived ATA exists!")
			return b.GetTokenAccountBalance(ctx, ata)
		}
	}

	// If derived ATA doesn't exist, try to find the actual account by mint
	log.Printf("[SPL] GetTokenBalance: derived ATA not found, searching via getTokenAccountsByOwner...")
	result, err := b.rpcClient.call(ctx, "getTokenAccountsByOwner", owner, map[string]interface{}{
		"programId": TokenProgramID,
	}, map[string]interface{}{
		"encoding": "jsonParsed",
	})
	if err != nil {
		log.Printf("[SPL] GetTokenBalance ERROR: failed to query token accounts: %v", err)
		return 0, nil
	}

	log.Printf("[SPL] GetTokenBalance: getTokenAccountsByOwner response: %s", string(result))

	type TokenAccountInfo struct {
		Mint        string `json:"mint"`
		TokenAmount struct {
			Amount string `json:"amount"`
		} `json:"tokenAmount"`
	}
	type TokenAccount struct {
		Pubkey  string `json:"pubkey"`
		Account struct {
			Data struct {
				Parsed struct {
					Info TokenAccountInfo `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"account"`
	}
	type RPCResponse struct {
		Value []TokenAccount `json:"value"`
	}

	var tokenAccounts RPCResponse
	if err := json.Unmarshal(result, &tokenAccounts); err != nil {
		log.Printf("[SPL] GetTokenBalance ERROR: parse failed: %v", err)
		return 0, nil
	}

	log.Printf("[SPL] GetTokenBalance: found %d token accounts", len(tokenAccounts.Value))
	for i, acc := range tokenAccounts.Value {
		amountStr := acc.Account.Data.Parsed.Info.TokenAmount.Amount
		log.Printf("[SPL] GetTokenBalance: account[%d] pubkey=%s, mint=%s, amount=%s",
			i, acc.Pubkey, acc.Account.Data.Parsed.Info.Mint, amountStr)

		if acc.Account.Data.Parsed.Info.Mint == mint {
			// Found matching mint
			var amount uint64
			fmt.Sscanf(amountStr, "%d", &amount)
			log.Printf("[SPL] GetTokenBalance: FOUND MATCHING TOKEN ACCOUNT! pubkey=%s, amount=%d",
				acc.Pubkey, amount)
			return amount, nil
		}
	}

	log.Printf("[SPL] GetTokenBalance: no token account found for mint=%s", mint)
	return 0, nil
}

// FindSPLTokenAccount finds the actual token account address for a wallet and mint
func (b *TxBuilder) FindSPLTokenAccount(ctx context.Context, owner, mint string) (string, error) {
	result, err := b.rpcClient.call(ctx, "getTokenAccountsByOwner", owner, map[string]interface{}{
		"programId": TokenProgramID,
	}, map[string]interface{}{
		"encoding": "jsonParsed",
	})
	if err != nil {
		return "", fmt.Errorf("failed to query token accounts: %w", err)
	}

	type TokenAccountInfo struct {
		Mint        string `json:"mint"`
		TokenAmount struct {
			Amount string `json:"amount"`
		} `json:"tokenAmount"`
	}
	type TokenAccount struct {
		Pubkey  string `json:"pubkey"`
		Account struct {
			Data struct {
				Parsed struct {
					Info TokenAccountInfo `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"account"`
	}
	type RPCResponse struct {
		Value []TokenAccount `json:"value"`
	}

	var tokenAccounts RPCResponse
	if err := json.Unmarshal(result, &tokenAccounts); err != nil {
		return "", fmt.Errorf("failed to parse token accounts: %w", err)
	}

	for _, acc := range tokenAccounts.Value {
		if acc.Account.Data.Parsed.Info.Mint == mint {
			return acc.Pubkey, nil
		}
	}

	return "", fmt.Errorf("token account not found for mint %s", mint)
}

// BuildSPLTransferTx builds an SPL token transfer transaction
func (b *TxBuilder) BuildSPLTransferTx(ctx context.Context, from, to, mint string, amount uint64, sourceATA string) ([]byte, error) {
	log.Printf("[SPL] BuildSPLTransferTx: from=%s, to=%s, mint=%s, amount=%d, sourceATA=%s", from, to, mint, amount, sourceATA)

	blockhash, err := b.rpcClient.GetLatestBlockhash(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blockhash: %w", err)
	}

	// Use provided source ATA or find it via RPC
	fromAta := sourceATA
	if fromAta == "" {
		fromAta, err = b.FindSPLTokenAccount(ctx, from, mint)
		if err != nil {
			return nil, fmt.Errorf("failed to find source token account: %w", err)
		}
		if fromAta == "" {
			return nil, fmt.Errorf("source wallet has no token account for mint %s", mint)
		}
	}
	log.Printf("[SPL] Source token account: %s", fromAta)

	// For destination, try to find existing token account
	toAta, err := b.FindSPLTokenAccount(ctx, to, mint)
	if err != nil {
		log.Printf("[SPL] Warning: Failed to find destination token account: %v", err)
	}

	if toAta == "" {
		log.Printf("[SPL] Destination has no token account for mint %s. Need to create ATA.", mint)
		// For now, we'll return an error. In production, you'd create the ATA here.
		return nil, fmt.Errorf("destination wallet has no token account for mint %s - ATA creation not yet implemented", mint)
	}
	log.Printf("[SPL] Destination token account: %s", toAta)

	// Create the transfer instruction data
	transferData := b.createSPLTransferData(amount)
	log.Printf("[SPL] Transfer instruction data length: %d", len(transferData))

	// Build account keys list - deduplicate if self-transfer
	accountKeys := []string{from}
	accountIndexMap := map[string]int{
		from: 0,
	}

	addAccount := func(addr string) uint8 {
		if idx, exists := accountIndexMap[addr]; exists {
			return uint8(idx)
		}
		idx := len(accountKeys)
		accountKeys = append(accountKeys, addr)
		accountIndexMap[addr] = idx
		return uint8(idx)
	}

	fromAtaIdx := addAccount(fromAta)
	toAtaIdx := addAccount(toAta)
	_ = addAccount(TokenProgramID)

	log.Printf("[SPL] Account keys: %v", accountKeys)
	log.Printf("[SPL] Indices: from=%d, fromAta=%d, toAta=%d, TokenProgram=%d",
		0, fromAtaIdx, toAtaIdx, accountIndexMap[TokenProgramID])

	msg := &Message{
		Header: MessageHeader{
			NumRequiredSignatures:       1,
			NumReadonlySignedAccounts:   0,
			NumReadonlyUnsignedAccounts: 2, // token program and potentially system
		},
		AccountKeys:     accountKeys,
		RecentBlockhash: blockhash.Blockhash,
		Instructions: []CompiledInstruction{
			{
				ProgramIDIndex: uint8(accountIndexMap[TokenProgramID]),
				// SPL Transfer instruction accounts: [source, dest, authority]
				Accounts: []uint8{fromAtaIdx, toAtaIdx, 0}, // source ATA, dest ATA, authority (from)
				Data:     transferData,
			},
		},
	}

	txBytes, err := b.serializeMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}
	log.Printf("[SPL] Transaction built: %d bytes", len(txBytes))

	return txBytes, nil
}

func (b *TxBuilder) createSPLTransferData(amount uint64) []byte {
	data := make([]byte, 9)
	data[0] = 3 // Transfer instruction index for SPL Token
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
