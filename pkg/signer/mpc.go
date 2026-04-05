package signer

import (
	"errors"
)

// MPCSigner is a placeholder for threshold signature (MPC) signing
// This will be implemented to integrate with your Rust MPC enclave
type MPCSigner struct {
	address string
}

// NewMPCSigner creates a new MPC signer placeholder
// In production, this will connect to the MPC enclave
func NewMPCSigner() *MPCSigner {
	return &MPCSigner{
		address: "0xMPCPLACEHOLDER0000000000000000000000",
	}
}

// Sign signs a transaction using MPC (placeholder)
// TODO: Implement actual MPC signing via FROST/GG20
func (s *MPCSigner) Sign(tx *Transaction) ([]byte, error) {
	return nil, errors.New("MPC signer not implemented - placeholder for threshold signatures. This will integrate with your Rust MPC enclave.")
}

// SignMessage signs an arbitrary message using MPC (placeholder)
func (s *MPCSigner) SignMessage(msg []byte) ([]byte, error) {
	return nil, errors.New("MPC signer not implemented - placeholder for threshold signatures")
}

// PublicKey returns a placeholder public key
// In production, this will be the aggregate public key from DKG
func (s *MPCSigner) PublicKey() []byte {
	return []byte("MPC_PLACEHOLDER_PUBLIC_KEY_65_BYTES_XXX")
}

// EthereumAddress returns the MPC aggregate address
func (s *MPCSigner) EthereumAddress() string {
	return s.address
}

// Zeroize is a no-op for MPC signer since keys are not stored locally
// In production, this would clear any cached session data
func (s *MPCSigner) Zeroize() {
	// No-op: MPC keys are never stored in memory
	// They exist as shares across participants
}

// IsZeroized returns true (MPC signer is always "ready" since no local key)
func (s *MPCSigner) IsZeroized() bool {
	return true
}

// Type returns the signer type
func (s *MPCSigner) Type() string {
	return string(SignerTypeMPC)
}

// SetAddress sets the MPC aggregate address (called after DKG)
func (s *MPCSigner) SetAddress(address string) {
	s.address = address
}

// Note for future implementation:
//
// To integrate with your Rust MPC enclave:
// 1. Add FFI bindings to call the enclave
// 2. Use your existing FROST-Ed25519 or switch to FROST-ECDSA
// 3. The sign method should:
//    - Send transaction hash to all MPC participants
//    - Collect partial signatures
//    - Aggregate into final signature
// 4. The public key should come from the DKG setup phase
//
// Example integration pattern:
//
// func (s *MPCSigner) Sign(tx *Transaction) ([]byte, error) {
//     // 1. Create signature share request
//     shareReq := MPCSignRequest{
//         Message: tx.Hash(),
//         Threshold: s.threshold,
//         Participants: s.participants,
//     }
//
//     // 2. Send to enclave
//     response, err := s.enclave.Sign(shareReq)
//     if err != nil {
//         return nil, err
//     }
//
//     // 3. Aggregate shares
//     return s.aggregator.Aggregate(response.Shares)
// }
