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
	IntentStatusDraft         IntentStatus = "draft"
	IntentStatusPending       IntentStatus = "pending"
	IntentStatusApproved      IntentStatus = "approved"
	IntentStatusExecuting     IntentStatus = "executing"
	IntentStatusSigned        IntentStatus = "signed"
	IntentStatusSent          IntentStatus = "sent"
	IntentStatusFailed        IntentStatus = "failed"
	IntentStatusPermanentFail IntentStatus = "permanent_fail"
	IntentStatusRejected      IntentStatus = "rejected"
	IntentStatusExpired       IntentStatus = "expired"
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
	ExpiresAt    time.Time    `json:"expires_at,omitempty"`
	ApprovedBy   []string     `json:"approved_by,omitempty"`
	RequiredSigs int          `json:"required_sigs"`
	TxHash       string       `json:"tx_hash,omitempty"`
	Error        string       `json:"error,omitempty"`
	RetryCount   int          `json:"retry_count"`
	MaxRetries   int          `json:"max_retries"`
	FeeSpeed     string       `json:"fee_speed"` // slow, standard, fast
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
func (s *IntentStore) CreateIntent(walletID, chain, from, to, value, valueETH string, gasLimit uint64, requiredSigs int, expiryHours int, maxRetries int, feeSpeed string) (*TransactionIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if expiryHours <= 0 {
		expiryHours = 24 // Default 24 hours
	}
	if maxRetries <= 0 {
		maxRetries = 3 // Default 3 retries
	}
	if feeSpeed == "" {
		feeSpeed = "standard" // Default fee speed
	}

	expiresAt := time.Now().Add(time.Duration(expiryHours) * time.Hour)

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
		ExpiresAt:    expiresAt,
		RequiredSigs: requiredSigs,
		MaxRetries:   maxRetries,
		RetryCount:   0,
		FeeSpeed:     feeSpeed,
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

// RetryIntent increments retry count and resets status for failed intents
// Returns error if intent cannot be retried (not failed, or max retries exceeded)
func (s *IntentStore) RetryIntent(id string) (*TransactionIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, err := s.getIntentUnsafe(id)
	if err != nil {
		return nil, err
	}

	if intent.Status != IntentStatusFailed && intent.Status != IntentStatusPermanentFail {
		return nil, fmt.Errorf("intent cannot be retried in status: %s", intent.Status)
	}

	// Check if max retries exceeded
	if intent.RetryCount >= intent.MaxRetries {
		return nil, fmt.Errorf("max retries (%d) exceeded for intent", intent.MaxRetries)
	}

	// Increment retry count
	intent.RetryCount++
	intent.UpdatedAt = time.Now()

	// Reset to approved status for re-execution
	// If multi-sig, go back to draft to require re-approval
	if intent.RequiredSigs > 1 {
		intent.Status = IntentStatusDraft
	} else {
		intent.Status = IntentStatusApproved
	}

	// Clear previous error
	intent.Error = ""

	if err := s.saveIntent(intent); err != nil {
		return nil, fmt.Errorf("failed to save intent: %w", err)
	}

	return intent, nil
}

// ExpireIntents expires draft/approved intents that have passed their expiry time
// Returns count of expired intents
func (s *IntentStore) ExpireIntents() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read intent dir: %w", err)
	}

	now := time.Now()
	expiredCount := 0

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

		// Only expire draft or approved intents
		if intent.Status != IntentStatusDraft && intent.Status != IntentStatusPending && intent.Status != IntentStatusApproved {
			continue
		}

		// Check if expired
		if !intent.ExpiresAt.IsZero() && now.After(intent.ExpiresAt) {
			intent.Status = IntentStatusExpired
			intent.UpdatedAt = now
			intent.Error = "Intent expired due to timeout"

			if err := s.saveIntent(&intent); err == nil {
				expiredCount++
			}
		}
	}

	return expiredCount, nil
}

// GetExpirableIntents returns intents that can be expired
func (s *IntentStore) GetExpirableIntents() ([]TransactionIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read intent dir: %w", err)
	}

	now := time.Now()
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

		// Only include draft, pending, or approved
		if intent.Status != IntentStatusDraft && intent.Status != IntentStatusPending && intent.Status != IntentStatusApproved {
			continue
		}

		// Check if expired
		if !intent.ExpiresAt.IsZero() && now.After(intent.ExpiresAt) {
			intents = append(intents, intent)
		}
	}

	return intents, nil
}

// IntentFilters holds filters for querying intents
type IntentFilters struct {
	WalletID  string
	Status    IntentStatus
	FromTime  time.Time
	ToTime    time.Time
	Chain     string
	Limit     int
	Offset    int
	SortBy    string // "created_at", "updated_at"
	SortOrder string // "asc", "desc"
}

// IntentStatusCounts holds count of intents by status for a wallet
type IntentStatusCounts struct {
	Pending   int `json:"pending"`
	Approved  int `json:"approved"`
	Executing int `json:"executing"`
	Sent      int `json:"sent"`
	Failed    int `json:"failed"`
	Rejected  int `json:"rejected"`
	Expired   int `json:"expired"`
	Draft     int `json:"draft"`
}

// GetIntentStatusCounts returns count of intents grouped by status for a wallet
func (s *IntentStore) GetIntentStatusCounts(walletID string) (*IntentStatusCounts, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read intent dir: %w", err)
	}

	counts := &IntentStatusCounts{}

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

		// Filter by wallet if specified
		if walletID != "" && intent.WalletID != walletID {
			continue
		}

		switch intent.Status {
		case IntentStatusDraft:
			counts.Draft++
		case IntentStatusPending:
			counts.Pending++
		case IntentStatusApproved:
			counts.Approved++
		case IntentStatusExecuting:
			counts.Executing++
		case IntentStatusSent:
			counts.Sent++
		case IntentStatusFailed, IntentStatusPermanentFail:
			counts.Failed++
		case IntentStatusRejected:
			counts.Rejected++
		case IntentStatusExpired:
			counts.Expired++
		}
	}

	return counts, nil
}

// ListIntentsFiltered returns intents matching the given filters
func (s *IntentStore) ListIntentsFiltered(filters IntentFilters) ([]TransactionIntent, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read intent dir: %w", err)
	}

	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.SortBy == "" {
		filters.SortBy = "created_at"
	}
	if filters.SortOrder == "" {
		filters.SortOrder = "desc"
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

		// Apply filters
		if filters.WalletID != "" && intent.WalletID != filters.WalletID {
			continue
		}
		if filters.Status != "" && intent.Status != filters.Status {
			continue
		}
		if filters.Chain != "" && intent.Chain != filters.Chain {
			continue
		}
		if !filters.FromTime.IsZero() && intent.CreatedAt.Before(filters.FromTime) {
			continue
		}
		if !filters.ToTime.IsZero() && intent.CreatedAt.After(filters.ToTime) {
			continue
		}

		intents = append(intents, intent)
	}

	// Sort
	for i := 0; i < len(intents)-1; i++ {
		for j := 0; j < len(intents)-i-1; j++ {
			var t1, t2 time.Time
			if filters.SortBy == "updated_at" {
				t1 = intents[j].UpdatedAt
				t2 = intents[j+1].UpdatedAt
			} else {
				t1 = intents[j].CreatedAt
				t2 = intents[j+1].CreatedAt
			}
			shouldSwap := false
			if filters.SortOrder == "asc" {
				shouldSwap = t1.After(t2)
			} else {
				shouldSwap = t1.Before(t2)
			}
			if shouldSwap {
				intents[j], intents[j+1] = intents[j+1], intents[j]
			}
		}
	}

	total := len(intents)

	// Apply pagination
	if filters.Offset > len(intents) {
		intents = []TransactionIntent{}
	} else {
		end := filters.Offset + filters.Limit
		if end > len(intents) {
			end = len(intents)
		}
		intents = intents[filters.Offset:end]
	}

	return intents, total, nil
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
