package signer

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/mr-tron/base58"
)

type SignCoordinator interface {
	Sign(ctx context.Context, msg []byte, unsignedTx []byte, rpcURL string) ([]byte, string, error) // returns (signature, txHash, error)
}

type MPCSolanaSigner struct {
	mu          sync.RWMutex
	nodeID      uint32
	clusterID   string
	publicKey   ed25519.PublicKey
	share       []byte
	connected   bool
	coordinator SignCoordinator
	rpcURL      string
}

func NewMPCSolanaSigner(nodeID uint32, clusterID string, coordinator SignCoordinator) (*MPCSolanaSigner, error) {
	return &MPCSolanaSigner{
		nodeID:      nodeID,
		clusterID:   clusterID,
		coordinator: coordinator,
		connected:   false,
	}, nil
}

func (s *MPCSolanaSigner) SetRPCURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rpcURL = url
}

func (s *MPCSolanaSigner) GetRPCURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rpcURL
}

func (s *MPCSolanaSigner) IsMPCBroadcast() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rpcURL != "" && s.coordinator != nil
}

func (s *MPCSolanaSigner) SetShareAndPublicKey(share, publicKey []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.share = make([]byte, len(share))
	copy(s.share, share)
	s.publicKey = make([]byte, len(publicKey))
	copy(s.publicKey, publicKey)
	s.connected = true
}

func (s *MPCSolanaSigner) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

func (s *MPCSolanaSigner) SignTransaction(unsignedTx []byte) ([]byte, error) {
	s.mu.RLock()
	if s.coordinator == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("MPC signer not configured: no SignCoordinator set")
	}
	if !s.connected {
		s.mu.RUnlock()
		return nil, fmt.Errorf("MPC signer not initialized: no key share loaded. Run DKG first to generate key shares.")
	}
	rpcURL := s.rpcURL
	pubKey := make([]byte, len(s.publicKey))
	copy(pubKey, s.publicKey)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	signature, txHash, err := s.coordinator.Sign(ctx, unsignedTx, unsignedTx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("MPC signing failed: %w", err)
	}

	_ = pubKey

	if txHash != "" {
		return []byte(txHash), nil
	}
	return signature, nil
}

func (s *MPCSolanaSigner) SignMessage(msg []byte) ([]byte, error) {
	return s.SignTransaction(msg)
}

func (s *MPCSolanaSigner) Address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.publicKey == nil {
		return ""
	}
	return base58.Encode(s.publicKey)
}

func (s *MPCSolanaSigner) PublicKey() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.publicKey == nil {
		return nil
	}
	result := make([]byte, len(s.publicKey))
	copy(result, s.publicKey)
	return result
}

func (s *MPCSolanaSigner) CompressedPublicKey() []byte {
	return s.PublicKey()
}

func (s *MPCSolanaSigner) PublicKeyHex() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.publicKey == nil {
		return ""
	}
	return hex.EncodeToString(s.publicKey)
}

func (s *MPCSolanaSigner) PrivateKeyHex() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.share == nil {
		return ""
	}
	return hex.EncodeToString(s.share)
}

func (s *MPCSolanaSigner) GetShare() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.share == nil {
		return nil
	}
	result := make([]byte, len(s.share))
	copy(result, s.share)
	return result
}

func (s *MPCSolanaSigner) Zeroize() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.share != nil {
		for i := range s.share {
			s.share[i] = 0
		}
		s.share = nil
	}
	s.publicKey = nil
	s.connected = false
}

func (s *MPCSolanaSigner) IsZeroized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.share == nil
}

func (s *MPCSolanaSigner) Type() string {
	return "mpc_solana"
}

func (s *MPCSolanaSigner) EthereumAddress() string {
	return s.Address()
}

func (s *MPCSolanaSigner) GetPrivateKey() ed25519.PrivateKey {
	return nil
}

func (s *MPCSolanaSigner) GetNodeID() uint32 {
	return s.nodeID
}

func (s *MPCSolanaSigner) GetClusterID() string {
	return s.clusterID
}