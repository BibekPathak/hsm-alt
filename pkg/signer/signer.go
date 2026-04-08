package signer

import (
	"errors"
	"math/big"
)

// Transaction represents a transaction to be signed
type Transaction struct {
	Chain    string
	To       string
	Value    *big.Int
	GasLimit uint64
	Data     []byte // For contract calls
}

// Signer is the interface for all signing implementations
// This abstraction allows swapping between ECDSA and MPC signers
type Signer interface {
	// Sign signs a transaction and returns the signature
	Sign(tx *Transaction) ([]byte, error)

	// SignMessage signs an arbitrary message
	SignMessage(msg []byte) ([]byte, error)

	// PublicKey returns the uncompressed public key
	PublicKey() []byte

	// EthereumAddress returns the checksummed Ethereum address
	EthereumAddress() string

	// Zeroize securely clears private key material from memory
	Zeroize()

	// IsZeroized returns true if the private key has been cleared
	IsZeroized() bool

	// Type returns the signer type for logging/display
	Type() string
}

// SignerType represents the type of signer
type SignerType string

const (
	SignerTypeECDSA SignerType = "ecdsa"
	SignerTypeMPC   SignerType = "mpc"
)

// ErrNotImplemented is returned when a signer method is not yet implemented
var ErrNotImplemented = errors.New("not implemented - placeholder for future signer type")

// NewSigner creates a new signer of the specified type
func NewSigner(signerType SignerType) (Signer, error) {
	switch signerType {
	case SignerTypeECDSA:
		return NewECDSASigner()
	case SignerTypeMPC:
		return NewMPCSigner(), nil
	default:
		return nil, errors.New("unknown signer type")
	}
}

// NewSignerFromHex creates a new signer of the specified type from hex private key
func NewSignerFromHex(signerType SignerType, hexKey string) (Signer, error) {
	switch signerType {
	case SignerTypeECDSA:
		return NewECDSASignerFromHex(hexKey)
	case SignerTypeMPC:
		s := NewMPCSigner()
		return s, nil
	default:
		return nil, errors.New("unknown signer type")
	}
}
