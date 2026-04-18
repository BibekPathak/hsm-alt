package node

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4
	argon2KeyLen  = 32
	saltLen       = 16
	version       = 1
)

type ShareStore struct {
	baseDir string
}

type EncryptedShare struct {
	Version   int       `json:"version"`
	NodeID    string    `json:"node_id"`
	ClusterID string    `json:"cluster_id"`
	Share     string    `json:"share"`      // base64 encoded encrypted share
	PublicKey string    `json:"public_key"` // base64 encoded public key
	CreatedAt time.Time `json:"created_at"`
}

func NewShareStore(baseDir string) *ShareStore {
	return &ShareStore{
		baseDir: baseDir,
	}
}

func (s *ShareStore) GetSharePath(nodeID uint32) string {
	return filepath.Join(s.baseDir, fmt.Sprintf("node_%d", nodeID), "share.json")
}

func (s *ShareStore) SaveShare(nodeID uint32, clusterID string, share []byte, publicKey []byte, password string) error {
	if nodeID == 0 {
		return fmt.Errorf("node_id cannot be zero")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	encrypted, err := encryptShare(share, password)
	if err != nil {
		return fmt.Errorf("failed to encrypt share: %w", err)
	}

	shareFile := &EncryptedShare{
		Version:   version,
		NodeID:    fmt.Sprintf("node%d", nodeID),
		ClusterID: clusterID,
		Share:     encrypted,
		PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		CreatedAt: time.Now().UTC(),
	}

	// Ensure directory exists
	nodeDir := filepath.Join(s.baseDir, fmt.Sprintf("node_%d", nodeID))
	if err := os.MkdirAll(nodeDir, 0700); err != nil {
		return fmt.Errorf("failed to create node directory: %w", err)
	}

	data, err := json.MarshalIndent(shareFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal share: %w", err)
	}

	if err := os.WriteFile(s.GetSharePath(nodeID), data, 0600); err != nil {
		return fmt.Errorf("failed to write share file: %w", err)
	}

	return nil
}

func (s *ShareStore) LoadShare(nodeID uint32, password string) (clusterID string, share []byte, publicKey []byte, err error) {
	if nodeID == 0 {
		return "", nil, nil, fmt.Errorf("node_id cannot be zero")
	}
	if password == "" {
		return "", nil, nil, fmt.Errorf("password cannot be empty")
	}

	sharePath := s.GetSharePath(nodeID)
	data, err := os.ReadFile(sharePath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to read share file: %w", err)
	}

	var encryptedShare EncryptedShare
	if err := json.Unmarshal(data, &encryptedShare); err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse share file: %w", err)
	}

	if encryptedShare.Version != version {
		return "", nil, nil, fmt.Errorf("unsupported share version: got %d, want %d", encryptedShare.Version, version)
	}

	share, err = decryptShare(encryptedShare.Share, password)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to decrypt share: %w", err)
	}

	publicKey, err = base64.StdEncoding.DecodeString(encryptedShare.PublicKey)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	return encryptedShare.ClusterID, share, publicKey, nil
}

func (s *ShareStore) DeleteShare(nodeID uint32) error {
	if nodeID == 0 {
		return fmt.Errorf("node_id cannot be zero")
	}

	sharePath := s.GetSharePath(nodeID)
	if err := os.Remove(sharePath); err != nil {
		return fmt.Errorf("failed to delete share: %w", err)
	}

	return nil
}

func (s *ShareStore) ShareExists(nodeID uint32) bool {
	if nodeID == 0 {
		return false
	}
	sharePath := s.GetSharePath(nodeID)
	_, err := os.Stat(sharePath)
	return err == nil
}

func (s *ShareStore) GetClusterID(nodeID uint32, password string) (string, error) {
	if nodeID == 0 {
		return "", fmt.Errorf("node_id cannot be zero")
	}

	sharePath := s.GetSharePath(nodeID)
	data, err := os.ReadFile(sharePath)
	if err != nil {
		return "", fmt.Errorf("failed to read share file: %w", err)
	}

	var encryptedShare EncryptedShare
	if err := json.Unmarshal(data, &encryptedShare); err != nil {
		return "", fmt.Errorf("failed to parse share file: %w", err)
	}

	return encryptedShare.ClusterID, nil
}

// encryptShare encrypts share using Argon2id + AES-256-GCM
func encryptShare(share []byte, password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, share, nil)

	// Combine salt + ciphertext
	result := append(salt, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}

func decryptShare(encrypted string, password string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	if len(data) < saltLen+1 {
		return nil, fmt.Errorf("encrypted data too short")
	}

	salt := data[:saltLen]
	ciphertext := data[saltLen:]

	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}