package wallet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store manages wallet persistence
type Store struct {
	baseDir string
	mu      sync.RWMutex
}

// NewStore creates a new wallet store
func NewStore(baseDir string) (*Store, error) {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		baseDir = filepath.Join(home, ".hsm", "wallets")
	}

	// Create directory structure
	dirs := []string{
		baseDir,
		filepath.Join(baseDir, "accounts"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create dir %s: %w", dir, err)
		}
	}

	return &Store{baseDir: baseDir}, nil
}

// CreateWallet creates a new wallet and returns it
func (s *Store) CreateWallet(name string) (*Wallet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wallet := &Wallet{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: now(),
	}

	// Save wallet
	data, err := json.MarshalIndent(wallet, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal wallet: %w", err)
	}

	walletPath := filepath.Join(s.baseDir, wallet.ID+".json")
	if err := os.WriteFile(walletPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write wallet: %w", err)
	}

	// Update index
	if err := s.updateIndex(wallet.ID); err != nil {
		return nil, err
	}

	return wallet, nil
}

// SaveAccount saves an account to storage
func (s *Store) SaveAccount(account *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(account, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal account: %w", err)
	}

	accountPath := filepath.Join(s.baseDir, "accounts", account.WalletID+"_"+account.Chain+".json")
	if err := os.WriteFile(accountPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write account: %w", err)
	}

	return nil
}

// GetWallet retrieves a wallet by ID
func (s *Store) GetWallet(id string) (*Wallet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	walletPath := filepath.Join(s.baseDir, id+".json")
	data, err := os.ReadFile(walletPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("wallet not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read wallet: %w", err)
	}

	var wallet Wallet
	if err := json.Unmarshal(data, &wallet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal wallet: %w", err)
	}

	return &wallet, nil
}

// GetAccount retrieves an account for a wallet and chain
func (s *Store) GetAccount(walletID, chain string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accountPath := filepath.Join(s.baseDir, "accounts", walletID+"_"+chain+".json")
	data, err := os.ReadFile(accountPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("account not found for %s on %s", walletID, chain)
		}
		return nil, fmt.Errorf("failed to read account: %w", err)
	}

	var account Account
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account: %w", err)
	}

	return &account, nil
}

// GetAccountsForWallet retrieves all accounts for a wallet
func (s *Store) GetAccountsForWallet(walletID string) ([]Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accountDir := filepath.Join(s.baseDir, "accounts")
	entries, err := os.ReadDir(accountDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read accounts dir: %w", err)
	}

	prefix := walletID + "_"
	var accounts []Account

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		if !containsPrefix(entry.Name(), prefix) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(accountDir, entry.Name()))
		if err != nil {
			continue
		}

		var account Account
		if err := json.Unmarshal(data, &account); err != nil {
			continue
		}

		accounts = append(accounts, account)
	}

	return accounts, nil
}

// ListWallets returns all wallet IDs
func (s *Store) ListWallets() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	indexPath := filepath.Join(s.baseDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	var walletIDs []string
	if err := json.Unmarshal(data, &walletIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	return walletIDs, nil
}

// updateIndex updates the wallet index file
func (s *Store) updateIndex(walletID string) error {
	indexPath := filepath.Join(s.baseDir, "index.json")

	var walletIDs []string
	if data, err := os.ReadFile(indexPath); err == nil {
		_ = json.Unmarshal(data, &walletIDs)
	}

	// Check if already exists
	for _, id := range walletIDs {
		if id == walletID {
			return nil
		}
	}

	walletIDs = append(walletIDs, walletID)

	data, err := json.MarshalIndent(walletIDs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

func now() time.Time {
	return time.Now().UTC()
}

func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
