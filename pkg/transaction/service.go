package transaction

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/yourorg/hsm/pkg/blockchain/ethereum"
	"github.com/yourorg/hsm/pkg/signer"
)

// Service orchestrates transaction building, signing, and broadcasting
type Service struct {
	builders map[string]*ethereum.TxBuilder // chain -> builder
}

// NewService creates a new transaction service
func NewService() *Service {
	return &Service{
		builders: make(map[string]*ethereum.TxBuilder),
	}
}

// AddChain adds a blockchain adapter
func (s *Service) AddChain(chain string, builder *ethereum.TxBuilder) {
	s.builders[chain] = builder
}

// SendTransaction builds, signs, and broadcasts a transaction
func (s *Service) SendTransaction(ctx context.Context, chain string, to string, value *big.Int, ecdsaSigner *signer.ECDSASigner) (string, error) {
	builder, ok := s.builders[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	// Get private key from signer
	privateKey := ecdsaSigner.GetPrivateKey()

	// Build and sign transaction
	_, rawTxHex, err := builder.BuildAndSignTx(ctx, to, value, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to build/sign tx: %w", err)
	}

	// Broadcast
	txHash, err := builder.BroadcastTransaction(ctx, rawTxHex)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return txHash, nil
}

// GetBalance gets the balance for an address on a chain
func (s *Service) GetBalance(ctx context.Context, chain string, address string) (*big.Int, error) {
	builder, ok := s.builders[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported chain: %s", chain)
	}

	return builder.GetBalance(ctx, address)
}

// CreateWallet creates a new keypair and returns the address
func CreateWallet() (*signer.ECDSASigner, string, error) {
	// Generate ECDSA key pair
	ecdsaSigner, err := signer.NewECDSASigner()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	address := ecdsaSigner.EthereumAddress()
	return ecdsaSigner, address, nil
}

// DeriveAddressFromPrivateKey derives an address from a private key
func DeriveAddressFromPrivateKey(privateKeyHex string) (string, error) {
	key, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	return crypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}

// AddressFromPublicKey derives an address from a public key
func AddressFromPublicKey(pubKey []byte) (string, error) {
	// This is a placeholder - real implementation would use Keccak256
	if len(pubKey) != 65 {
		return "", fmt.Errorf("expected 65 bytes uncompressed public key")
	}

	// Extract the actual public key without the 0x04 prefix
	ethKey := pubKey[1:]

	hash := crypto.Keccak256(ethKey)
	// Take last 20 bytes
	address := hash[12:]

	return fmt.Sprintf("0x%x", address), nil
}
