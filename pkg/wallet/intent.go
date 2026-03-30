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

// IntentStatus represents the status of a transaction intent
type IntentStatus string

const (
	IntentStatusDraft    IntentStatus = "draft"
	IntentStatusPending  IntentStatus = "pending"
	IntentStatusApproved IntentStatus = "approved"
	IntentStatusSigned   IntentStatus = "signed"
	IntentStatusSent     IntentStatus = "sent"
	IntentStatusFailed   IntentStatus = "failed"
	IntentStatusRejected IntentStatus = "rejected"
)

// TransactionIntent represents an intent to send a transaction
type TransactionIntent struct {
	ID           string       `json:"id"`
	WalletID     string       `json:"wallet_id"`
	Chain        string       `json:"chain"`
	From         string       `json:"from"`
	To           string       `json:"to"`
	Value        string       `json:"value"`     // In wei
	ValueETH     string       `json:"value_eth"` // Human readable
	Data         string       `json:"data,omitempty"`
	GasLimit     uint64       `json:"gas_limit"`
	Nonce        uint64       `json:"nonce,omitempty"`
	Status       IntentStatus `json:"status"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	ApprovedBy   []string     `json:"approved_by,omitempty"`
	RequiredSigs int          `json:"required_sigs"`
	TxHash       string       `json:"tx_hash,omitempty"`
	Error        string       `json:"error,omitempty"`
}

// IntentStore manages transaction intents for audit trail and draft approvals
type IntentStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewIntentStore creates a new intent store
func NewIntentStore(baseDir string) (*IntentStore, error) {
	intentDir := filepath.Join(baseDir, "intents")
	if err := os.MkdirAll(intentDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create intent dir: %w", err)
	}

	return &IntentStore{baseDir: intentDir}, nil
}

// CreateIntent creates a new transaction intent
func (s *IntentStore) CreateIntent(walletID, chain, from, to, value, valueETH string, gasLimit uint64, requiredSigs int) (*TransactionIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent := &TransactionIntent{
		ID:           uuid.New().String(),
		WalletID:     walletID,
		Chain:        chain,
		From:         from,
		To:           to,
		Value:        value,
		ValueETH:     valueETH,
		GasLimit:     gasLimit,
		Status:       IntentStatusDraft,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		RequiredSigs: requiredSigs,
	}

	if err := s.saveIntent(intent); err != nil {
		return nil, fmt.Errorf("failed to save intent: %w", err)
	}

	return intent, nil
}

// GetIntent retrieves an intent by ID
func (s *IntentStore) GetIntent(id string) (*TransactionIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	intentPath := s.intentPath(id)
	data, err := os.ReadFile(intentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("intent not found")
		}
		return nil, fmt.Errorf("failed to read intent: %w", err)
	}

	var intent TransactionIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intent: %w", err)
	}

	return &intent, nil
}

// ListIntents lists all intents for a wallet
func (s *IntentStore) ListIntents(walletID string) ([]TransactionIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read intent dir: %w", err)
	}

	var intents []TransactionIntent
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.baseDir, entry.Name()))
		if err != nil {
			continue
		}

		var intent TransactionIntent
		if err := json.Unmarshal(data, &intent); err != nil {
			continue
		}

		if walletID == "" || intent.WalletID == walletID {
			intents = append(intents, intent)
		}
	}

	return intents, nil
}

// ApproveIntent adds an approval to an intent
func (s *IntentStore) ApproveIntent(id, approver string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, err := s.getIntentUnsafe(id)
	if err != nil {
		return err
	}

	if intent.Status != IntentStatusDraft && intent.Status != IntentStatusPending {
		return fmt.Errorf("intent cannot be approved in status: %s", intent.Status)
	}

	// Check if already approved by this approver
	for _, a := range intent.ApprovedBy {
		if a == approver {
			return fmt.Errorf("already approved by %s", approver)
		}
	}

	intent.ApprovedBy = append(intent.ApprovedBy, approver)
	intent.UpdatedAt = time.Now()

	// Check if we have enough approvals
	if len(intent.ApprovedBy) >= intent.RequiredSigs {
		intent.Status = IntentStatusApproved
	}

	return s.saveIntent(intent)
}

// UpdateIntentStatus updates the status of an intent
func (s *IntentStore) UpdateIntentStatus(id string, status IntentStatus, txHash, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, err := s.getIntentUnsafe(id)
	if err != nil {
		return err
	}

	intent.Status = status
	intent.UpdatedAt = time.Now()
	if txHash != "" {
		intent.TxHash = txHash
	}
	if errMsg != "" {
		intent.Error = errMsg
	}

	return s.saveIntent(intent)
}

// GetPendingIntents returns all intents pending approval
func (s *IntentStore) GetPendingIntents() ([]TransactionIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read intent dir: %w", err)
	}

	var intents []TransactionIntent
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.baseDir, entry.Name()))
		if err != nil {
			continue
		}

		var intent TransactionIntent
		if err := json.Unmarshal(data, &intent); err != nil {
			continue
		}

		if intent.Status == IntentStatusDraft || intent.Status == IntentStatusPending {
			intents = append(intents, intent)
		}
	}

	return intents, nil
}

func (s *IntentStore) getIntentUnsafe(id string) (*TransactionIntent, error) {
	intentPath := s.intentPath(id)
	data, err := os.ReadFile(intentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("intent not found")
		}
		return nil, fmt.Errorf("failed to read intent: %w", err)
	}

	var intent TransactionIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intent: %w", err)
	}

	return &intent, nil
}

func (s *IntentStore) saveIntent(intent *TransactionIntent) error {
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal intent: %w", err)
	}

	intentPath := s.intentPath(intent.ID)
	if err := os.WriteFile(intentPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write intent: %w", err)
	}

	return nil
}

func (s *IntentStore) intentPath(id string) string {
	return filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
}
