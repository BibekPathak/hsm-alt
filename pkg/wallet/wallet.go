package wallet

import (
	"time"
)

// Wallet is the logical container for multi-chain accounts
type Wallet struct {
	ID        string    `json:"wallet_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Account is the chain-specific address (1 wallet -> N accounts)
type Account struct {
	WalletID   string `json:"wallet_id"`
	Chain      string `json:"chain"`       // "ethereum", "solana"
	Address    string `json:"address"`     // 0x... or base58...
	PubKey     []byte `json:"pubkey"`      // Compressed pubkey
	SignerType string `json:"signer_type"` // "ecdsa" or "mpc"
	Index      uint32 `json:"index"`       // For future HD wallets
}

// CreateWalletRequest is the API request to create a wallet
type CreateWalletRequest struct {
	Name string `json:"name"`
}

// CreateWalletResponse is the API response for wallet creation
type CreateWalletResponse struct {
	WalletID string    `json:"wallet_id"`
	Name     string    `json:"name"`
	Accounts []Account `json:"accounts"`
}

// WalletInfoResponse is the API response for wallet info
type WalletInfoResponse struct {
	Wallet   Wallet    `json:"wallet"`
	Accounts []Account `json:"accounts"`
}

// SendTxRequest is the API request to send a transaction
type SendTxRequest struct {
	WalletID string `json:"wallet_id"`
	To       string `json:"to"`
	Value    string `json:"value"`
	Chain    string `json:"chain"` // Optional, defaults to wallet's primary chain
}

// SendTxResponse is the API response for send transaction
type SendTxResponse struct {
	TxHash string `json:"tx_hash"`
	Chain  string `json:"chain"`
}

// BalanceResponse is the API response for balance
type BalanceResponse struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
	Chain   string `json:"chain"`
}

// CreateAddressRequest is the API request to add a new address to a wallet
type CreateAddressRequest struct {
	Chain string `json:"chain"` // "ethereum", "solana"
}

// CreateAddressResponse is the API response for address creation
type CreateAddressResponse struct {
	WalletID string  `json:"wallet_id"`
	Account  Account `json:"account"`
}

// DeleteWalletResponse is the API response for wallet deletion
type DeleteWalletResponse struct {
	WalletID string `json:"wallet_id"`
	Deleted  bool   `json:"deleted"`
	Message  string `json:"message"`
}

// DeleteAccountResponse is the API response for account deletion
type DeleteAccountResponse struct {
	WalletID string `json:"wallet_id"`
	Chain    string `json:"chain"`
	Index    uint32 `json:"index"`
	Deleted  bool   `json:"deleted"`
	Message  string `json:"message"`
}
