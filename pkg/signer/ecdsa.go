package signer

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// Signer interface for signing operations
type Signer interface {
	Sign(hash []byte) ([]byte, error)
	PublicKey() []byte
	CompressedPublicKey() []byte
	EthereumAddress() string
}

// ECDSASigner provides secp256k1 ECDSA signing for Ethereum transactions
type ECDSASigner struct {
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
}

// NewECDSASigner creates a new secp256k1 ECDSA signer
func NewECDSASigner() (*ECDSASigner, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	return &ECDSASigner{
		privateKey: key,
		publicKey:  &key.PublicKey,
	}, nil
}

// NewECDSASignerFromHex creates a signer from a hex-encoded private key
func NewECDSASignerFromHex(hexKey string) (*ECDSASigner, error) {
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &ECDSASigner{
		privateKey: key,
		publicKey:  &key.PublicKey,
	}, nil
}

// NewECDSASignerFromBytes creates a signer from raw private key bytes
func NewECDSASignerFromBytes(keyBytes []byte) (*ECDSASigner, error) {
	key, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &ECDSASigner{
		privateKey: key,
		publicKey:  &key.PublicKey,
	}, nil
}

// Sign signs a hash using ECDSA
func (s *ECDSASigner) Sign(hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be 32 bytes, got %d", len(hash))
	}

	sig, err := crypto.Sign(hash, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	return sig, nil
}

// PublicKey returns the uncompressed public key (65 bytes)
func (s *ECDSASigner) PublicKey() []byte {
	return crypto.FromECDSAPub(s.publicKey)
}

// CompressedPublicKey returns the compressed public key (33 bytes)
func (s *ECDSASigner) CompressedPublicKey() []byte {
	return crypto.CompressPubkey(s.publicKey)
}

// EthereumAddress returns the checksummed Ethereum address
func (s *ECDSASigner) EthereumAddress() string {
	return crypto.PubkeyToAddress(*s.publicKey).Hex()
}

// GetPrivateKey returns the ECDSA private key for transaction signing
func (s *ECDSASigner) GetPrivateKey() *ecdsa.PrivateKey {
	return s.privateKey
}

// PrivateKeyHex returns the hex-encoded private key (for backup)
func (s *ECDSASigner) PrivateKeyHex() string {
	return hex.EncodeToString(crypto.FromECDSA(s.privateKey))
}

// SignTransaction signs a pre-hashed transaction
func (s *ECDSASigner) SignTransaction(txHash []byte) ([]byte, []byte, error) {
	sig, err := crypto.Sign(txHash, s.privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// sig is [R || S || V] (65 bytes)
	r := sig[:32]
	sBytes := sig[32:64]
	return r, sBytes, nil
}

// VerifySignature verifies a signature against a hash and public key
func VerifySignature(pubKey, hash, signature []byte) bool {
	if len(hash) != 32 || len(signature) != 65 {
		return false
	}

	return crypto.VerifySignature(pubKey, hash, signature[:64])
}
