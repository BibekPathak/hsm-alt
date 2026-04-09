package signer

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/mr-tron/base58"
)

type SolanaSigner struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

func NewSolanaSigner() (*SolanaSigner, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	return &SolanaSigner{
		privateKey: priv,
		publicKey:  pub,
	}, nil
}

func NewSolanaSignerFromSeed(seed []byte) (*SolanaSigner, error) {
	if len(seed) != 32 {
		return nil, fmt.Errorf("seed must be 32 bytes")
	}

	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)

	return &SolanaSigner{
		privateKey: priv,
		publicKey:  pub,
	}, nil
}

func NewSolanaSignerFromHex(hexKey string) (*SolanaSigner, error) {
	seed, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	if len(seed) == 64 {
		seed = seed[:32]
	}

	return NewSolanaSignerFromSeed(seed)
}

func (s *SolanaSigner) SignMessage(msg []byte) ([]byte, error) {
	return ed25519.Sign(s.privateKey, msg), nil
}

func (s *SolanaSigner) SignTransaction(txMessage []byte) ([]byte, error) {
	return ed25519.Sign(s.privateKey, txMessage), nil
}

func (s *SolanaSigner) Address() string {
	return base58.Encode(s.publicKey)
}

func (s *SolanaSigner) EthereumAddress() string {
	return base58.Encode(s.publicKey)
}

func (s *SolanaSigner) PublicKey() []byte {
	return s.publicKey
}

func (s *SolanaSigner) CompressedPublicKey() []byte {
	return s.publicKey
}

func (s *SolanaSigner) PublicKeyHex() string {
	return hex.EncodeToString(s.publicKey)
}

func (s *SolanaSigner) PrivateKeyHex() string {
	return hex.EncodeToString(s.privateKey[:32])
}

func (s *SolanaSigner) GetPrivateKey() ed25519.PrivateKey {
	return s.privateKey
}

func (s *SolanaSigner) Zeroize() {
	if s.privateKey != nil {
		for i := range s.privateKey {
			s.privateKey[i] = 0
		}
	}
	s.privateKey = nil
	s.publicKey = nil
}

func (s *SolanaSigner) IsZeroized() bool {
	return s.privateKey == nil
}

func (s *SolanaSigner) Type() string {
	return string(SignerTypeSolana)
}

func (s *SolanaSigner) Sign(tx *Transaction) ([]byte, error) {
	sig, err := s.SignMessage(tx.Data)
	return sig, err
}

func VerifySolanaSignature(pubKey []byte, message []byte, signature []byte) bool {
	return ed25519.Verify(pubKey, message, signature)
}
