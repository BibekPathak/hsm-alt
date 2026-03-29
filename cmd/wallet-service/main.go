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
)

type Server struct {
	walletStore  *wallet.Store
	txService    *transaction.Service
	ecdsaSigners map[string]*signer.ECDSASigner // wallet_id -> signer
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

	// Initialize wallet store
	walletStore, err := wallet.NewStore(storageDir)
	if err != nil {
		log.Fatalf("Failed to initialize wallet store: %v", err)
	}

	// Initialize transaction service
	txService := transaction.NewService()

	// Initialize Ethereum builder
	rpcClient := ethereum.NewRPCClient(rpcURL)
	builder := ethereum.NewTxBuilder(rpcClient, chainID)
	txService.AddChain("ethereum", builder)

	// Initialize server
	server := &Server{
		walletStore:  walletStore,
		txService:    txService,
		ecdsaSigners: make(map[string]*signer.ECDSASigner),
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
	r.Get("/wallet/{id}/balance", server.handleGetBalance)
	r.Post("/wallet/{id}/send", server.handleSendTx)

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

	// Store signer in memory
	s.ecdsaSigners[newWallet.ID] = ecdsaSigner

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

	var wallets []wallet.WalletInfoResponse
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

	// Create unique signer key for this account
	signerKey := fmt.Sprintf("%s_%s_%d", walletID, req.Chain, nextIndex)
	s.ecdsaSigners[signerKey] = ecdsaSigner

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

	log.Printf("Created address %s for wallet %s (index %d)", account.Address, walletID, nextIndex)

	sendJSON(w, http.StatusCreated, wallet.CreateAddressResponse{
		WalletID: walletID,
		Account:  *account,
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

	// Get signer for wallet
	ecdsaSigner, exists := s.ecdsaSigners[walletID]
	if !exists {
		sendError(w, http.StatusNotFound, "Wallet not found or not loaded")
		return
	}

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
