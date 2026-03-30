package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/yourorg/hsm/pkg/blockchain/ethereum"
	"github.com/yourorg/hsm/pkg/signer"
	"github.com/yourorg/hsm/pkg/transaction"
	"github.com/yourorg/hsm/pkg/wallet"
)

const (
	defaultPort = "8080"
	defaultRPC  = "https://sepolia.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161" // Public Sepolia RPC
	chainID     = 11155111                                                        // Sepolia
	defaultPass = "hsm-default-password"                                          // Default password for key encryption
)

type Server struct {
	walletStore *wallet.Store
	keyStore    *wallet.KeyStore
	intentStore *wallet.IntentStore
	txService   *transaction.Service
	password    string // Default encryption password
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	rpcURL := os.Getenv("ETHEREUM_RPC")
	if rpcURL == "" {
		rpcURL = defaultRPC
	}

	storageDir := os.Getenv("WALLET_DIR")
	if storageDir == "" {
		storageDir = "~/.hsm/wallets"
	}

	password := os.Getenv("ENCRYPTION_PASSWORD")
	if password == "" {
		password = defaultPass
	}

	// Initialize wallet store
	walletStore, err := wallet.NewStore(storageDir)
	if err != nil {
		log.Fatalf("Failed to initialize wallet store: %v", err)
	}

	// Initialize key store for encrypted private key storage
	keyStore, err := wallet.NewKeyStore(storageDir)
	if err != nil {
		log.Fatalf("Failed to initialize key store: %v", err)
	}

	// Initialize intent store for transaction history
	intentStore, err := wallet.NewIntentStore(storageDir)
	if err != nil {
		log.Fatalf("Failed to initialize intent store: %v", err)
	}

	// Initialize transaction service
	txService := transaction.NewService()

	// Initialize Ethereum builder
	rpcClient := ethereum.NewRPCClient(rpcURL)
	builder := ethereum.NewTxBuilder(rpcClient, chainID)
	txService.AddChain("ethereum", builder)

	// Initialize server
	server := &Server{
		walletStore: walletStore,
		keyStore:    keyStore,
		intentStore: intentStore,
		txService:   txService,
		password:    password,
	}

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	// Routes
	r.Post("/wallet/create", server.handleCreateWallet)
	r.Get("/wallet", server.handleListWallets)
	r.Get("/wallet/{id}", server.handleGetWallet)
	r.Get("/wallet/{id}/address", server.handleGetAddress)
	r.Post("/wallet/{id}/address", server.handleCreateAddress)
	r.Delete("/wallet/{id}", server.handleDeleteWallet)
	r.Delete("/wallet/{id}/address/{index}", server.handleDeleteAddress)
	r.Get("/wallet/{id}/balance", server.handleGetBalance)
	r.Post("/wallet/{id}/send", server.handleSendTx)
	r.Get("/wallet/{id}/summary", server.handleWalletSummary)

	// Intent routes
	r.Post("/intent", server.handleCreateIntent)
	r.Get("/intent", server.handleListIntents)
	r.Get("/intent/{id}", server.handleGetIntent)
	r.Post("/intent/{id}/approve", server.handleApproveIntent)
	r.Post("/intent/{id}/execute", server.handleExecuteIntent)

	log.Printf("Wallet Service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (s *Server) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	var req wallet.CreateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		req.Name = "Default Wallet"
	}

	// Create wallet
	newWallet, err := s.walletStore.CreateWallet(req.Name)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create wallet: %v", err))
		return
	}

	// Generate ECDSA keypair for Ethereum
	ecdsaSigner, err := signer.NewECDSASigner()
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate key: %v", err))
		return
	}

	// Create account
	account := &wallet.Account{
		WalletID:   newWallet.ID,
		Chain:      "ethereum",
		Address:    ecdsaSigner.EthereumAddress(),
		PubKey:     ecdsaSigner.CompressedPublicKey(),
		SignerType: "ecdsa",
		Index:      0,
	}

	if err := s.walletStore.SaveAccount(account); err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save account: %v", err))
		return
	}

	// Save encrypted private key to disk
	if err := s.keyStore.SaveKey(newWallet.ID, "ethereum", 0, account.Address, ecdsaSigner.PrivateKeyHex(), s.password); err != nil {
		log.Printf("Warning: Failed to save private key: %v", err)
	}

	// Zeroize private key from memory
	ecdsaSigner.Zeroize()

	// Return response
	response := wallet.CreateWalletResponse{
		WalletID: newWallet.ID,
		Name:     newWallet.Name,
		Accounts: []wallet.Account{*account},
	}

	log.Printf("Created wallet %s with address %s", newWallet.ID, account.Address)
	sendJSON(w, http.StatusCreated, response)
}

func (s *Server) handleListWallets(w http.ResponseWriter, r *http.Request) {
	walletIDs, err := s.walletStore.ListWallets()
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list wallets: %v", err))
		return
	}

	// Initialize as empty slice to avoid null JSON
	wallets := make([]wallet.WalletInfoResponse, 0)
	for _, id := range walletIDs {
		w, err := s.walletStore.GetWallet(id)
		if err != nil {
			continue
		}

		accounts, _ := s.walletStore.GetAccountsForWallet(id)
		wallets = append(wallets, wallet.WalletInfoResponse{
			Wallet:   *w,
			Accounts: accounts,
		})
	}

	sendJSON(w, http.StatusOK, wallets)
}

func (s *Server) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")

	wal, err := s.walletStore.GetWallet(walletID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Wallet not found: %v", err))
		return
	}

	accounts, err := s.walletStore.GetAccountsForWallet(walletID)
	if err != nil {
		accounts = []wallet.Account{}
	}

	sendJSON(w, http.StatusOK, wallet.WalletInfoResponse{
		Wallet:   *wal,
		Accounts: accounts,
	})
}

func (s *Server) handleGetAddress(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")

	account, err := s.walletStore.GetAccount(walletID, "ethereum")
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Account not found: %v", err))
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{
		"address": account.Address,
		"chain":   "ethereum",
	})
}

func (s *Server) handleCreateAddress(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")

	// Verify wallet exists
	_, err := s.walletStore.GetWallet(walletID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Wallet not found: %v", err))
		return
	}

	var req wallet.CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Chain == "" {
		req.Chain = "ethereum"
	}

	// Count existing accounts for this chain
	existingAccounts, _ := s.walletStore.GetAccountsForWallet(walletID)
	nextIndex := uint32(0)
	for _, acc := range existingAccounts {
		if acc.Chain == req.Chain && acc.Index >= nextIndex {
			nextIndex = acc.Index + 1
		}
	}

	// Generate new ECDSA keypair
	ecdsaSigner, err := signer.NewECDSASigner()
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate key: %v", err))
		return
	}

	// Create account
	account := &wallet.Account{
		WalletID:   walletID,
		Chain:      req.Chain,
		Address:    ecdsaSigner.EthereumAddress(),
		PubKey:     ecdsaSigner.CompressedPublicKey(),
		SignerType: "ecdsa",
		Index:      nextIndex,
	}

	if err := s.walletStore.SaveAccount(account); err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save account: %v", err))
		return
	}

	// Save encrypted private key to disk
	if err := s.keyStore.SaveKey(walletID, req.Chain, nextIndex, account.Address, ecdsaSigner.PrivateKeyHex(), s.password); err != nil {
		log.Printf("Warning: Failed to save private key: %v", err)
	}

	// Zeroize private key from memory
	ecdsaSigner.Zeroize()

	log.Printf("Created address %s for wallet %s (index %d)", account.Address, walletID, nextIndex)

	sendJSON(w, http.StatusCreated, wallet.CreateAddressResponse{
		WalletID: walletID,
		Account:  *account,
	})
}

func (s *Server) handleDeleteWallet(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")

	// Verify wallet exists
	_, err := s.walletStore.GetWallet(walletID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Wallet not found: %v", err))
		return
	}

	// Delete wallet and all accounts
	if err := s.walletStore.DeleteWallet(walletID); err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete wallet: %v", err))
		return
	}

	log.Printf("Deleted wallet %s", walletID)

	sendJSON(w, http.StatusOK, wallet.DeleteWalletResponse{
		WalletID: walletID,
		Deleted:  true,
		Message:  "Wallet and all associated accounts deleted",
	})
}

func (s *Server) handleDeleteAddress(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")
	indexStr := chi.URLParam(r, "index")

	// Parse index
	var index uint32
	if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid index parameter")
		return
	}

	// Verify wallet exists
	_, err := s.walletStore.GetWallet(walletID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Wallet not found: %v", err))
		return
	}

	// Delete account
	if err := s.walletStore.DeleteAccount(walletID, "ethereum", index); err != nil {
		if err.Error() == fmt.Sprintf("account not found for %s on ethereum at index %d", walletID, index) {
			sendError(w, http.StatusNotFound, err.Error())
			return
		}
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete account: %v", err))
		return
	}

	log.Printf("Deleted address at index %d for wallet %s", index, walletID)

	sendJSON(w, http.StatusOK, wallet.DeleteAccountResponse{
		WalletID: walletID,
		Chain:    "ethereum",
		Index:    index,
		Deleted:  true,
		Message:  "Address deleted",
	})
}

func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")

	account, err := s.walletStore.GetAccount(walletID, "ethereum")
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Account not found: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	balance, err := s.txService.GetBalance(ctx, "ethereum", account.Address)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get balance: %v", err))
		return
	}

	// Convert from wei to ETH
	ethBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt(big.NewInt(1e18)))

	sendJSON(w, http.StatusOK, wallet.BalanceResponse{
		Address: account.Address,
		Balance: ethBalance.Text('f', 6),
		Chain:   "ethereum",
	})
}

func (s *Server) handleSendTx(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")

	var req wallet.SendTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Load key from keystore and sign immediately, then zeroize
	// This is the "sign-and-forget" pattern - private key never stays in memory
	address, privateKeyHex, err := s.keyStore.LoadKey(walletID, "ethereum", 0, s.password)
	if err != nil {
		sendError(w, http.StatusNotFound, "Wallet not found or key not available")
		return
	}

	// Create signer for this operation
	ecdsaSigner, err := signer.NewECDSASignerFromHex(privateKeyHex)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to load signing key")
		return
	}

	// Verify address matches
	if ecdsaSigner.EthereumAddress() != address {
		sendError(w, http.StatusInternalServerError, "Address mismatch")
		return
	}

	// Zeroize the private key immediately after creating signer
	// Note: go-ethereum's ecdsa.PrivateKey uses big.Int internally
	// For extra security, we keep the signer but ensure it's zeroized after use
	defer ecdsaSigner.Zeroize()

	// Parse value from ETH to wei
	ethValue := new(big.Float)
	if _, ok := ethValue.SetString(req.Value); !ok {
		sendError(w, http.StatusBadRequest, "Invalid value format")
		return
	}

	weiValue := new(big.Int)
	ethValue.Mul(ethValue, new(big.Float).SetInt(big.NewInt(1e18)))
	ethValue.Int(weiValue)

	// Send transaction
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	txHash, err := s.txService.SendTransaction(ctx, "ethereum", req.To, weiValue, ecdsaSigner)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to send tx: %v", err))
		return
	}

	log.Printf("Sent tx %s from wallet %s to %s for %s ETH", txHash, walletID, req.To, req.Value)

	sendJSON(w, http.StatusOK, wallet.SendTxResponse{
		TxHash: txHash,
		Chain:  "ethereum",
	})
}

func (s *Server) handleWalletSummary(w http.ResponseWriter, r *http.Request) {
	walletID := chi.URLParam(r, "id")

	wal, err := s.walletStore.GetWallet(walletID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Wallet not found: %v", err))
		return
	}

	accounts, _ := s.walletStore.GetAccountsForWallet(walletID)

	// Get total balance across all addresses
	var totalBalance big.Int
	type addrBalance struct {
		Address string `json:"address"`
		Balance string `json:"balance"`
		Chain   string `json:"chain"`
	}
	var balances []addrBalance

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	for _, acc := range accounts {
		balance, err := s.txService.GetBalance(ctx, acc.Chain, acc.Address)
		if err != nil {
			balances = append(balances, addrBalance{
				Address: acc.Address,
				Balance: "error",
				Chain:   acc.Chain,
			})
			continue
		}
		totalBalance.Add(&totalBalance, balance)
		ethBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt(big.NewInt(1e18)))
		balances = append(balances, addrBalance{
			Address: acc.Address,
			Balance: ethBalance.Text('f', 6),
			Chain:   acc.Chain,
		})
	}

	totalETH := new(big.Float).Quo(new(big.Float).SetInt(&totalBalance), new(big.Float).SetInt(big.NewInt(1e18)))

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"wallet_id":     wal.ID,
		"name":          wal.Name,
		"total_balance": totalETH.Text('f', 6),
		"addresses":     balances,
	})
}

func (s *Server) handleCreateIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WalletID     string `json:"wallet_id"`
		Chain        string `json:"chain"`
		To           string `json:"to"`
		Value        string `json:"value"`
		GasLimit     uint64 `json:"gas_limit"`
		RequiredSigs int    `json:"required_sigs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get wallet's address
	account, err := s.walletStore.GetAccount(req.WalletID, req.Chain)
	if err != nil {
		sendError(w, http.StatusNotFound, "Account not found")
		return
	}

	// Create intent
	intent, err := s.intentStore.CreateIntent(
		req.WalletID,
		req.Chain,
		account.Address,
		req.To,
		req.Value,
		"", // value_eth will be calculated
		req.GasLimit,
		req.RequiredSigs,
	)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create intent: %v", err))
		return
	}

	log.Printf("Created intent %s for wallet %s", intent.ID, req.WalletID)
	sendJSON(w, http.StatusCreated, intent)
}

func (s *Server) handleListIntents(w http.ResponseWriter, r *http.Request) {
	walletID := r.URL.Query().Get("wallet_id")

	intents, err := s.intentStore.ListIntents(walletID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list intents: %v", err))
		return
	}

	if intents == nil {
		intents = []wallet.TransactionIntent{}
	}

	sendJSON(w, http.StatusOK, intents)
}

func (s *Server) handleGetIntent(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "id")

	intent, err := s.intentStore.GetIntent(intentID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Intent not found: %v", err))
		return
	}

	sendJSON(w, http.StatusOK, intent)
}

func (s *Server) handleApproveIntent(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "id")

	var req struct {
		Approver string `json:"approver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Approver == "" {
		req.Approver = "unknown"
	}

	if err := s.intentStore.ApproveIntent(intentID, req.Approver); err != nil {
		sendError(w, http.StatusBadRequest, fmt.Sprintf("Failed to approve intent: %v", err))
		return
	}

	intent, _ := s.intentStore.GetIntent(intentID)
	log.Printf("Intent %s approved by %s", intentID, req.Approver)
	sendJSON(w, http.StatusOK, intent)
}

func (s *Server) handleExecuteIntent(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "id")

	intent, err := s.intentStore.GetIntent(intentID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Intent not found: %v", err))
		return
	}

	if intent.Status != wallet.IntentStatusApproved {
		sendError(w, http.StatusBadRequest, "Intent not approved")
		return
	}

	// Load key and sign
	_, privateKeyHex, err := s.keyStore.LoadKey(intent.WalletID, intent.Chain, 0, s.password)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to load key")
		return
	}

	ecdsaSigner, err := signer.NewECDSASignerFromHex(privateKeyHex)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Failed to create signer")
		return
	}
	defer ecdsaSigner.Zeroize()

	// Parse value
	ethValue := new(big.Float)
	ethValue.SetString(intent.Value)
	weiValue := new(big.Int)
	ethValue.Mul(ethValue, new(big.Float).SetInt(big.NewInt(1e18)))
	ethValue.Int(weiValue)

	// Send transaction
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	txHash, err := s.txService.SendTransaction(ctx, intent.Chain, intent.To, weiValue, ecdsaSigner)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", err.Error())
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to send tx: %v", err))
		return
	}

	// Update intent status
	s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusSent, txHash, "")

	log.Printf("Executed intent %s, tx hash: %s", intentID, txHash)
	sendJSON(w, http.StatusOK, map[string]string{
		"tx_hash":   txHash,
		"intent_id": intentID,
	})
}

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func sendError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
