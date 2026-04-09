package solana

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/mr-tron/base58"
)

const (
	SystemProgramID = "11111111111111111111111111111111"
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
	data := make([]byte, 4+binary.MaxVarintLen64)
	binary.LittleEndian.PutUint32(data[0:4], 2)

	n := binary.PutUvarint(data[4:], lamports)
	return data[:4+n]
}

func (b *TxBuilder) serializeMessage(msg *Message) ([]byte, error) {
	var buf bytes.Buffer

	numSignatures := byte(len(msg.AccountKeys[:msg.Header.NumRequiredSignatures]))
	buf.WriteByte(numSignatures)

	for i := 0; i < int(numSignatures); i++ {
		sig := make([]byte, 64)
		buf.Write(sig)
	}

	buf.WriteByte(byte(msg.Header.NumRequiredSignatures))
	buf.WriteByte(msg.Header.NumReadonlySignedAccounts)
	buf.WriteByte(msg.Header.NumReadonlyUnsignedAccounts)

	numAccountKeys := byte(len(msg.AccountKeys))
	buf.WriteByte(numAccountKeys)
	for _, key := range msg.AccountKeys {
		decoded, err := base58.Decode(key)
		if err != nil {
			return nil, fmt.Errorf("failed to decode account key: %w", err)
		}
		buf.Write(decoded)
	}

	blockhash, err := base58.Decode(msg.RecentBlockhash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode blockhash: %w", err)
	}
	buf.Write(blockhash)

	numInstructions := byte(len(msg.Instructions))
	buf.WriteByte(numInstructions)

	for _, inst := range msg.Instructions {
		buf.WriteByte(inst.ProgramIDIndex)
		numAccounts := byte(len(inst.Accounts))
		buf.WriteByte(numAccounts)
		buf.Write(inst.Accounts)
		dataLen := binary.PutUvarint(make([]byte, 0, 4), uint64(len(inst.Data)))
		buf.WriteByte(byte(dataLen))
		buf.Write(inst.Data)
	}

	return buf.Bytes(), nil
}

func (b *TxBuilder) AddSignature(unsignedTx []byte, signature []byte) ([]byte, error) {
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	var buf bytes.Buffer
	buf.Write(unsignedTx)

	buf.WriteByte(1)

	sigBuf := make([]byte, 64)
	copy(sigBuf, signature)
	buf.Write(sigBuf)

	return buf.Bytes(), nil
}

func (b *TxBuilder) SerializeWithSignatures(msg *Message, signatures [][]byte) ([]byte, error) {
	var buf bytes.Buffer

	numSignatures := byte(len(signatures))
	buf.WriteByte(numSignatures)

	for _, sig := range signatures {
		sig64 := make([]byte, 64)
		copy(sig64, sig)
		buf.Write(sig64)
	}

	buf.WriteByte(byte(msg.Header.NumRequiredSignatures))
	buf.WriteByte(msg.Header.NumReadonlySignedAccounts)
	buf.WriteByte(msg.Header.NumReadonlyUnsignedAccounts)

	numAccountKeys := byte(len(msg.AccountKeys))
	buf.WriteByte(numAccountKeys)
	for _, key := range msg.AccountKeys {
		decoded, err := base58.Decode(key)
		if err != nil {
			return nil, fmt.Errorf("failed to decode account key: %w", err)
		}
		buf.Write(decoded)
	}

	blockhash, err := base58.Decode(msg.RecentBlockhash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode blockhash: %w", err)
	}
	buf.Write(blockhash)

	numInstructions := byte(len(msg.Instructions))
	buf.WriteByte(numInstructions)

	for _, inst := range msg.Instructions {
		buf.WriteByte(inst.ProgramIDIndex)
		numAccounts := byte(len(inst.Accounts))
		buf.WriteByte(numAccounts)
		buf.Write(inst.Accounts)
		dataLen := binary.PutUvarint(make([]byte, 0, 4), uint64(len(inst.Data)))
		buf.WriteByte(byte(dataLen))
		buf.Write(inst.Data)
	}

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
