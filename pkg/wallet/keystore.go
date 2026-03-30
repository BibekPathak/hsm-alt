package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// KeyStore manages encrypted private key storage
type KeyStore struct {
	baseDir string
	mu      sync.RWMutex
}

// StoredKey represents an encrypted private key
type StoredKey struct {
	WalletID string `json:"wallet_id"`
	Chain    string `json:"chain"`
	Index    uint32 `json:"index"`
	Address  string `json:"address"`
	KeyHex   string `json:"key_hex"` // Encrypted private key
}

// NewKeyStore creates a new key store
func NewKeyStore(baseDir string) (*KeyStore, error) {
	keyDir := filepath.Join(baseDir, "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key dir: %w", err)
	}

	return &KeyStore{baseDir: keyDir}, nil
}

// SaveKey saves an encrypted private key
func (ks *KeyStore) SaveKey(walletID, chain string, index uint32, address string, privateKeyHex string, password string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	// Encrypt the private key
	encrypted, err := encryptKey(privateKeyHex, password)
	if err != nil {
		return fmt.Errorf("failed to encrypt key: %w", err)
	}

	stored := StoredKey{
		WalletID: walletID,
		Chain:    chain,
		Index:    index,
		Address:  address,
		KeyHex:   encrypted,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key: %w", err)
	}

	keyPath := ks.keyPath(walletID, chain, index)
	if err := os.WriteFile(keyPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	return nil
}

// LoadKey loads and decrypts a private key
func (ks *KeyStore) LoadKey(walletID, chain string, index uint32, password string) (string, string, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	keyPath := ks.keyPath(walletID, chain, index)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("key not found")
		}
		return "", "", fmt.Errorf("failed to read key: %w", err)
	}

	var stored StoredKey
	if err := json.Unmarshal(data, &stored); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal key: %w", err)
	}

	// Decrypt the private key
	privateKeyHex, err := decryptKey(stored.KeyHex, password)
	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt key: %w", err)
	}

	return stored.Address, privateKeyHex, nil
}

// ListKeys lists all stored keys for a wallet
func (ks *KeyStore) ListKeys(walletID string) ([]StoredKey, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	entries, err := os.ReadDir(ks.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read key dir: %w", err)
	}

	prefix := walletID + "_"
	var keys []StoredKey

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		if !containsPrefix(entry.Name(), prefix) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(ks.baseDir, entry.Name()))
		if err != nil {
			continue
		}

		var stored StoredKey
		if err := json.Unmarshal(data, &stored); err != nil {
			continue
		}

		keys = append(keys, stored)
	}

	return keys, nil
}

// DeleteKey deletes a stored key
func (ks *KeyStore) DeleteKey(walletID, chain string, index uint32) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	keyPath := ks.keyPath(walletID, chain, index)
	if err := os.Remove(keyPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("key not found")
		}
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

// keyPath returns the path to a key file
func (ks *KeyStore) keyPath(walletID, chain string, index uint32) string {
	return filepath.Join(ks.baseDir, fmt.Sprintf("%s_%s_%d.json", walletID, chain, index))
}

// encryptKey encrypts a private key hex string with AES-GCM
func encryptKey(privateKeyHex, password string) (string, error) {
	// Derive key from password
	key := deriveKey(password)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(privateKeyHex), nil)

	return hex.EncodeToString(ciphertext), nil
}

// decryptKey decrypts an encrypted private key hex string
func decryptKey(encryptedHex, password string) (string, error) {
	// Decode hex
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	// Derive key from password
	key := deriveKey(password)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// deriveKey derives a 32-byte key from a password using SHA-256
func deriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}
