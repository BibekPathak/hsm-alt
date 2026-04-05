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

	"golang.org/x/crypto/argon2"
)

// Key version constants
const (
	KeyVersionV1 = 1 // SHA-256 (legacy)
	KeyVersionV2 = 2 // Argon2id
)

// Argon2 defaults
const (
	Argon2Time        = uint32(3)
	Argon2MemoryKB    = uint32(65536) // 64MB
	Argon2Parallelism = uint8(4)
	Argon2KeyLength   = uint32(32)
	Argon2SaltLength  = 16
)

// KeyStore manages encrypted private key storage
type KeyStore struct {
	baseDir string
	mu      sync.RWMutex
}

// StoredKey represents an encrypted private key (legacy version 1)
type StoredKey struct {
	Version  int    `json:"version,omitempty"` // 1 or absent for legacy
	KDF      string `json:"kdf,omitempty"`     // "sha256" or absent for legacy
	WalletID string `json:"wallet_id"`
	Chain    string `json:"chain"`
	Index    uint32 `json:"index"`
	Address  string `json:"address"`
	KeyHex   string `json:"key_hex"` // Encrypted private key
}

// StoredKeyV2 represents an encrypted private key with Argon2id
type StoredKeyV2 struct {
	Version     int    `json:"version"` // 2
	KDF         string `json:"kdf"`     // "argon2id"
	WalletID    string `json:"wallet_id"`
	Chain       string `json:"chain"`
	Index       uint32 `json:"index"`
	Address     string `json:"address"`
	TimeCost    uint32 `json:"time"`        // Argon2 time parameter
	MemoryCost  uint32 `json:"memory_kb"`   // Argon2 memory parameter
	Parallelism uint8  `json:"parallelism"` // Argon2 parallelism parameter
	Salt        string `json:"salt"`        // hex-encoded salt
	KeyHex      string `json:"key_hex"`     // Encrypted private key
}

// NewKeyStore creates a new key store
func NewKeyStore(baseDir string) (*KeyStore, error) {
	keyDir := filepath.Join(baseDir, "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key dir: %w", err)
	}

	return &KeyStore{baseDir: keyDir}, nil
}

// SaveKey saves an encrypted private key using Argon2id
func (ks *KeyStore) SaveKey(walletID, chain string, index uint32, address string, privateKeyHex string, password string) error {
	return ks.SaveKeyV2(walletID, chain, index, address, privateKeyHex, password)
}

// SaveKeyV2 saves an encrypted private key using Argon2id (new format)
func (ks *KeyStore) SaveKeyV2(walletID, chain string, index uint32, address string, privateKeyHex string, password string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	// Generate random salt
	salt := make([]byte, Argon2SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key using Argon2id
	derivedKey := argon2.IDKey(
		[]byte(password),
		salt,
		Argon2Time,
		Argon2MemoryKB,
		Argon2Parallelism,
		Argon2KeyLength,
	)

	// Encrypt with AES-GCM
	encrypted, err := encryptWithKey(derivedKey, []byte(privateKeyHex))
	if err != nil {
		return fmt.Errorf("failed to encrypt key: %w", err)
	}

	stored := StoredKeyV2{
		Version:     KeyVersionV2,
		KDF:         "argon2id",
		WalletID:    walletID,
		Chain:       chain,
		Index:       index,
		Address:     address,
		TimeCost:    Argon2Time,
		MemoryCost:  Argon2MemoryKB,
		Parallelism: Argon2Parallelism,
		Salt:        hex.EncodeToString(salt),
		KeyHex:      hex.EncodeToString(encrypted),
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

// LoadKey loads and decrypts a private key, with lazy migration from SHA-256 to Argon2id
func (ks *KeyStore) LoadKey(walletID, chain string, index uint32, password string) (string, string, error) {
	ks.mu.RLock()
	keyPath := ks.keyPath(walletID, chain, index)
	data, err := os.ReadFile(keyPath)
	ks.mu.RUnlock()

	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("key not found")
		}
		return "", "", fmt.Errorf("failed to read key: %w", err)
	}

	// Try to detect version
	var versionCheck struct {
		Version int    `json:"version"`
		KDF     string `json:"kdf"`
	}
	if err := json.Unmarshal(data, &versionCheck); err != nil {
		return "", "", fmt.Errorf("failed to parse key metadata: %w", err)
	}

	var address, privateKeyHex string
	var needsMigration bool

	switch {
	case versionCheck.Version == KeyVersionV2 && versionCheck.KDF == "argon2id":
		// New Argon2id format
		var stored StoredKeyV2
		if err := json.Unmarshal(data, &stored); err != nil {
			return "", "", fmt.Errorf("failed to unmarshal key: %w", err)
		}
		address = stored.Address
		privateKeyHex, err = decryptKeyV2(&stored, password)

	case versionCheck.Version == KeyVersionV1 || versionCheck.KDF == "sha256" || (versionCheck.Version == 0 && versionCheck.KDF == ""):
		// Legacy SHA-256 format
		var stored StoredKey
		if err := json.Unmarshal(data, &stored); err != nil {
			return "", "", fmt.Errorf("failed to unmarshal key: %w", err)
		}
		address = stored.Address
		privateKeyHex, err = decryptKeyV1(stored.KeyHex, password)
		if err == nil {
			needsMigration = true
		}

	default:
		return "", "", fmt.Errorf("unknown key version: %d", versionCheck.Version)
	}

	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt key: %w", err)
	}

	// Lazy migration: re-encrypt with Argon2id
	if needsMigration {
		go func() {
			if migrateErr := ks.SaveKeyV2(walletID, chain, index, address, privateKeyHex, password); migrateErr == nil {
				fmt.Printf("Migrated key %s_%s_%d from SHA-256 to Argon2id\n", walletID, chain, index)
			}
		}()
	}

	return address, privateKeyHex, nil
}

// ListKeys lists all stored keys for a wallet
func (ks *KeyStore) ListKeys(walletID string) ([]StoredKeyV2, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	entries, err := os.ReadDir(ks.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read key dir: %w", err)
	}

	prefix := walletID + "_"
	var keys []StoredKeyV2

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

		var stored StoredKeyV2
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

// encryptWithKey encrypts data using AES-GCM with a pre-derived key
func encryptWithKey(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptWithKey decrypts data using AES-GCM with a pre-derived key
func decryptWithKey(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// decryptKeyV2 decrypts a private key using Argon2id (new format)
func decryptKeyV2(stored *StoredKeyV2, password string) (string, error) {
	salt, err := hex.DecodeString(stored.Salt)
	if err != nil {
		return "", fmt.Errorf("invalid salt: %w", err)
	}

	ciphertext, err := hex.DecodeString(stored.KeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext: %w", err)
	}

	// Derive key using Argon2id with stored parameters
	derivedKey := argon2.IDKey(
		[]byte(password),
		salt,
		stored.TimeCost,
		stored.MemoryCost,
		stored.Parallelism,
		Argon2KeyLength,
	)

	plaintext, err := decryptWithKey(derivedKey, ciphertext)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// decryptKeyV1 decrypts a private key using SHA-256 (legacy format)
func decryptKeyV1(encryptedHex, password string) (string, error) {
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	key := deriveKeySHA256(password)
	plaintext, err := decryptWithKey(key, ciphertext)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// deriveKeySHA256 derives a 32-byte key from a password using SHA-256 (legacy)
func deriveKeySHA256(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}
