package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/yourorg/hsm/pkg/blockchain/ethereum"
	"github.com/yourorg/hsm/pkg/blockchain/solana"
	"github.com/yourorg/hsm/pkg/signer"
	"github.com/yourorg/hsm/pkg/transaction"
	"github.com/yourorg/hsm/pkg/wallet"
)

const (
	defaultPort         = "8080"
	defaultRPC          = "https://sepolia.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161" // Public Sepolia RPC
	chainID             = 11155111                                                        // Sepolia
	defaultPass         = "hsm-default-password"                                          // Default password for key encryption
	defaultSolanaRPCURL = "https://api.devnet.solana.com"                                 // Solana Devnet
)

const (
	defaultIntentExpiryHours = 24
	defaultMaxRetries        = 3
	defaultLockTimeout       = 30 * time.Second
)

type Server struct {
	walletStore       *wallet.Store
	keyStore          *wallet.KeyStore
	intentStore       *wallet.IntentStore
	txService         *transaction.Service
	password          string
	intentExpiryHours int
	maxRetries        int
	lockTimeout       time.Duration
	walletLocks       sync.Map // map of "walletID_chain" -> struct{}
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

	// Configurable intent expiry hours
	intentExpiryHours := defaultIntentExpiryHours
	if expiryStr := os.Getenv("INTENT_EXPIRY_HOURS"); expiryStr != "" {
		if expiry, err := strconv.Atoi(expiryStr); err == nil && expiry > 0 {
			intentExpiryHours = expiry
		}
	}

	// Configurable max retries
	maxRetries := defaultMaxRetries
	if retriesStr := os.Getenv("INTENT_MAX_RETRIES"); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil && retries > 0 {
			maxRetries = retries
		}
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

	// Initialize Solana builder
	solanaRPCURL := os.Getenv("SOLANA_RPC")
	if solanaRPCURL == "" {
		solanaRPCURL = defaultSolanaRPCURL
	}
	solanaRPCClient := solana.NewRPCClient(solanaRPCURL)
	solanaBuilder := solana.NewTxBuilder(solanaRPCClient)
	txService.AddSolanaChain("solana", solanaBuilder)

	// Initialize server
	server := &Server{
		walletStore:       walletStore,
		keyStore:          keyStore,
		intentStore:       intentStore,
		txService:         txService,
		password:          password,
		intentExpiryHours: intentExpiryHours,
		maxRetries:        maxRetries,
		lockTimeout:       defaultLockTimeout,
	}

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	// CORS for frontend
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Routes
	r.Post("/wallet/create", server.handleCreateWallet)
	r.Get("/wallet", server.handleListWallets)
	r.Get("/wallet/{id}", server.handleGetWallet)
	r.Get("/wallet/{id}/address", server.handleGetAddress)
	r.Post("/wallet/{id}/address", server.handleCreateAddress)
	r.Delete("/wallet/{id}", server.handleDeleteWallet)
	r.Delete("/wallet/{id}/address/{index}", server.handleDeleteAddress)
	r.Get("/wallet/{id}/balance", server.handleGetBalance)
	r.Get("/wallet/{id}/summary", server.handleWalletSummary)

	// Intent routes - ALL transactions go through intent flow
	r.Post("/intent", server.handleCreateIntent)
	r.Get("/intent", server.handleListIntents)
	r.Get("/intent/{id}", server.handleGetIntent)
	r.Post("/intent/{id}/approve", server.handleApproveIntent)
	r.Post("/intent/{id}/reject", server.handleRejectIntent)
	r.Post("/intent/{id}/retry", server.handleRetryIntent)
	r.Post("/intent/{id}/execute", server.handleExecuteIntent)

	// Fee estimation
	r.Get("/fee-estimate", server.handleFeeEstimate)

	// Start background expiry job
	go server.runExpiryJob()

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

	newWallet, err := s.walletStore.CreateWallet(req.Name)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create wallet: %v", err))
		return
	}

	var accounts []wallet.Account

	ecdsaSigner, err := signer.NewECDSASigner()
	if err == nil {
		ethAccount := &wallet.Account{
			WalletID:   newWallet.ID,
			Chain:      "ethereum",
			Address:    ecdsaSigner.EthereumAddress(),
			PubKey:     ecdsaSigner.CompressedPublicKey(),
			SignerType: "ecdsa",
			Index:      0,
		}
		if err := s.walletStore.SaveAccount(ethAccount); err == nil {
			s.keyStore.SaveKey(newWallet.ID, "ethereum", 0, ethAccount.Address, ecdsaSigner.PrivateKeyHex(), s.password)
			accounts = append(accounts, *ethAccount)
		}
		ecdsaSigner.Zeroize()
	}

	solanaSigner, err := signer.NewSolanaSigner()
	if err == nil {
		solAccount := &wallet.Account{
			WalletID:   newWallet.ID,
			Chain:      "solana",
			Address:    solanaSigner.Address(),
			PubKey:     solanaSigner.CompressedPublicKey(),
			SignerType: "ed25519",
			Index:      0,
		}
		if err := s.walletStore.SaveAccount(solAccount); err == nil {
			s.keyStore.SaveKey(newWallet.ID, "solana", 0, solAccount.Address, solanaSigner.PrivateKeyHex(), s.password)
			accounts = append(accounts, *solAccount)
		}
		solanaSigner.Zeroize()
	}

	if len(accounts) == 0 {
		sendError(w, http.StatusInternalServerError, "Failed to create any accounts")
		return
	}

	response := wallet.CreateWalletResponse{
		WalletID: newWallet.ID,
		Name:     newWallet.Name,
		Accounts: accounts,
	}

	log.Printf("Created wallet %s with %d accounts", newWallet.ID, len(accounts))
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

	existingAccounts, _ := s.walletStore.GetAccountsForWallet(walletID)
	nextIndex := uint32(0)
	for _, acc := range existingAccounts {
		if acc.Chain == req.Chain && acc.Index >= nextIndex {
			nextIndex = acc.Index + 1
		}
	}

	var account *wallet.Account

	if req.Chain == "solana" {
		solanaSigner, err := signer.NewSolanaSigner()
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate key: %v", err))
			return
		}
		account = &wallet.Account{
			WalletID:   walletID,
			Chain:      req.Chain,
			Address:    solanaSigner.Address(),
			PubKey:     solanaSigner.CompressedPublicKey(),
			SignerType: "ed25519",
			Index:      nextIndex,
		}
		s.keyStore.SaveKey(walletID, req.Chain, nextIndex, account.Address, solanaSigner.PrivateKeyHex(), s.password)
		solanaSigner.Zeroize()
	} else {
		ecdsaSigner, err := signer.NewECDSASigner()
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate key: %v", err))
			return
		}
		account = &wallet.Account{
			WalletID:   walletID,
			Chain:      req.Chain,
			Address:    ecdsaSigner.EthereumAddress(),
			PubKey:     ecdsaSigner.CompressedPublicKey(),
			SignerType: "ecdsa",
			Index:      nextIndex,
		}
		s.keyStore.SaveKey(walletID, req.Chain, nextIndex, account.Address, ecdsaSigner.PrivateKeyHex(), s.password)
		ecdsaSigner.Zeroize()
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
	chain := r.URL.Query().Get("chain")
	if chain == "" {
		chain = "ethereum"
	}

	account, err := s.walletStore.GetAccount(walletID, chain)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Account not found: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	balance, err := s.txService.GetBalance(ctx, chain, account.Address)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get balance: %v", err))
		return
	}

	var balanceStr string
	if chain == "solana" {
		solBalance := float64(balance.Uint64()) / float64(solana.LamportsPerSOL)
		balanceStr = fmt.Sprintf("%.6f", solBalance)
	} else {
		ethBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt(big.NewInt(1e18)))
		balanceStr = ethBalance.Text('f', 6)
	}

	sendJSON(w, http.StatusOK, wallet.BalanceResponse{
		Address: account.Address,
		Balance: balanceStr,
		Chain:   chain,
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
		var balanceStr string
		if acc.Chain == "solana" {
			solBalance := float64(balance.Uint64()) / float64(solana.LamportsPerSOL)
			balanceStr = fmt.Sprintf("%.6f", solBalance)
		} else {
			ethBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt(big.NewInt(1e18)))
			balanceStr = ethBalance.Text('f', 6)
		}
		balances = append(balances, addrBalance{
			Address: acc.Address,
			Balance: balanceStr,
			Chain:   acc.Chain,
		})
	}

	totalETH := new(big.Float).Quo(new(big.Float).SetInt(&totalBalance), new(big.Float).SetInt(big.NewInt(1e18)))

	// Get intent counts for this wallet
	intentCounts, _ := s.intentStore.GetIntentStatusCounts(walletID)

	// Get fee estimates
	type feeEstimate struct {
		Native string `json:"native"`
		ERC20  string `json:"erc20"`
		SPL    string `json:"spl"`
	}
	fees := feeEstimate{
		Native: "21000 gas",
		ERC20:  "65000 gas",
		SPL:    "10000 lamports",
	}

	// Try to get actual fee estimates
	if builder, ok := s.txService.GetBuilder("ethereum"); ok {
		if feeInfo, err := builder.GetFeeInfo(ctx); err == nil {
			fees.Native = fmt.Sprintf("%.0f gas (%.2f gwei)", float64(21000), float64(feeInfo.Standard.Int64())/1e9)
		}
	}
	if solBuilder, ok := s.txService.GetSolanaBuilder("solana"); ok {
		if fee, err := solBuilder.GetFeeEstimate(ctx); err == nil {
			fees.SPL = fmt.Sprintf("%d lamports", fee)
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"wallet_id":     wal.ID,
		"name":          wal.Name,
		"total_balance": totalETH.Text('f', 6),
		"addresses":     balances,
		"intents":       intentCounts,
		"fees":          fees,
	})
}

func (s *Server) handleCreateIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WalletID      string `json:"wallet_id"`
		Chain         string `json:"chain"`
		To            string `json:"to"`
		Value         string `json:"value"`
		GasLimit      uint64 `json:"gas_limit"`
		RequiredSigs  int    `json:"required_sigs"`
		Creator       string `json:"creator"`                  // Who created this intent
		FeeSpeed      string `json:"fee_speed"`                // slow, standard, fast
		TokenAddress  string `json:"token_address,omitempty"`  // ERC-20 contract or SPL mint
		TokenDecimals uint8  `json:"token_decimals,omitempty"` // e.g., 6 for USDC
		TokenSymbol   string `json:"token_symbol,omitempty"`   // e.g., "USDC"
		TokenType     string `json:"token_type,omitempty"`     // "native", "erc20", "spl"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Default to ethereum chain
	if req.Chain == "" {
		req.Chain = "ethereum"
	}

	// Determine token type
	if req.TokenType == "" {
		if req.TokenAddress != "" {
			// Token address provided - infer token type from chain
			if req.Chain == "solana" {
				req.TokenType = "spl"
			} else {
				req.TokenType = "erc20"
			}
		} else {
			req.TokenType = "native"
		}
	}

	// Default gas limit based on token type
	if req.GasLimit == 0 {
		switch req.TokenType {
		case "erc20":
			req.GasLimit = 65000 // ERC-20 transfer
		case "spl":
			req.GasLimit = 10000 // SPL transfer (compute units)
		default:
			req.GasLimit = 21000 // Native ETH transfer
		}
	}

	// Default required_sigs to 1 for fast path
	if req.RequiredSigs <= 0 {
		req.RequiredSigs = 1
	}

	// Default creator to "creator" if not specified
	if req.Creator == "" {
		req.Creator = "creator"
	}

	// Get wallet's address
	account, err := s.walletStore.GetAccount(req.WalletID, req.Chain)
	if err != nil {
		sendError(w, http.StatusNotFound, "Account not found")
		return
	}

	// Create display value
	valueDisplay := req.Value
	if req.TokenSymbol != "" {
		valueDisplay = fmt.Sprintf("%s %s", req.Value, req.TokenSymbol)
	} else if req.Chain == "solana" {
		valueDisplay = fmt.Sprintf("%s SOL", req.Value)
	} else if req.Chain == "ethereum" && req.TokenType == "native" {
		valueDisplay = fmt.Sprintf("%s ETH", req.Value)
	}

	// Create intent
	intent, err := s.intentStore.CreateIntent(
		req.WalletID,
		req.Chain,
		account.Address,
		req.To,
		req.Value,
		valueDisplay,
		req.GasLimit,
		req.RequiredSigs,
		s.intentExpiryHours,
		s.maxRetries,
		req.FeeSpeed,
	)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create intent: %v", err))
		return
	}

	// Update token fields
	if req.TokenAddress != "" || req.TokenType != "native" {
		intent.TokenAddress = req.TokenAddress
		intent.TokenDecimals = req.TokenDecimals
		intent.TokenSymbol = req.TokenSymbol
		intent.TokenType = req.TokenType
		s.intentStore.UpdateIntent(intent)
	}

	// Auto-approve if required_sigs is 1 (fast path)
	if req.RequiredSigs == 1 {
		if err := s.intentStore.ApproveIntent(intent.ID, req.Creator); err != nil {
			log.Printf("Warning: Failed to auto-approve intent: %v", err)
		} else {
			// Reload intent to get updated status
			intent, _ = s.intentStore.GetIntent(intent.ID)
			log.Printf("Auto-approved intent %s (required_sigs=1)", intent.ID)
		}
	}

	log.Printf("Created intent %s for wallet %s (required_sigs: %d, token: %s)", intent.ID, req.WalletID, req.RequiredSigs, req.TokenType)
	sendJSON(w, http.StatusCreated, intent)
}

func (s *Server) handleListIntents(w http.ResponseWriter, r *http.Request) {
	filters := wallet.IntentFilters{
		WalletID:  r.URL.Query().Get("wallet_id"),
		Chain:     r.URL.Query().Get("chain"),
		SortBy:    r.URL.Query().Get("sort"),
		SortOrder: r.URL.Query().Get("order"),
	}

	// Status filter
	if status := r.URL.Query().Get("status"); status != "" {
		filters.Status = wallet.IntentStatus(status)
	}

	// Date filters
	if from := r.URL.Query().Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filters.FromTime = t
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filters.ToTime = t
		}
	}

	// Pagination
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filters.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filters.Offset = o
		}
	}

	intents, total, err := s.intentStore.ListIntentsFiltered(filters)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list intents: %v", err))
		return
	}

	if intents == nil {
		intents = []wallet.TransactionIntent{}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"intents": intents,
		"total":   total,
		"limit":   filters.Limit,
		"offset":  filters.Offset,
	})
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

func (s *Server) handleRejectIntent(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "id")

	var req struct {
		Rejecter string `json:"rejecter"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Rejecter == "" {
		req.Rejecter = "unknown"
	}

	intent, err := s.intentStore.GetIntent(intentID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Intent not found: %v", err))
		return
	}

	// Can only reject draft or pending intents
	if intent.Status != wallet.IntentStatusDraft && intent.Status != wallet.IntentStatusPending {
		sendError(w, http.StatusBadRequest, "Can only reject draft or pending intents")
		return
	}

	if err := s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusRejected, "", req.Reason); err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reject intent: %v", err))
		return
	}

	log.Printf("Intent %s rejected by %s: %s", intentID, req.Rejecter, req.Reason)
	sendJSON(w, http.StatusOK, map[string]string{
		"intent_id": intentID,
		"status":    string(wallet.IntentStatusRejected),
		"reason":    req.Reason,
	})
}

func (s *Server) handleExecuteIntent(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "id")

	intent, err := s.intentStore.GetIntent(intentID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Intent not found: %v", err))
		return
	}

	if intent.Status == wallet.IntentStatusExecuting {
		sendError(w, http.StatusConflict, "Intent is already being executed")
		return
	}
	if intent.Status == wallet.IntentStatusSent {
		sendError(w, http.StatusConflict, fmt.Sprintf("Intent already executed with tx: %s", intent.TxHash))
		return
	}

	if intent.Status != wallet.IntentStatusApproved {
		sendError(w, http.StatusBadRequest, "Intent not approved")
		return
	}

	lockKey := intent.WalletID + "_" + intent.Chain
	lockAcquired := make(chan struct{})

	go func() {
		s.walletLocks.LoadOrStore(lockKey, struct{}{})
		close(lockAcquired)
	}()

	select {
	case <-lockAcquired:
	case <-time.After(s.lockTimeout):
		sendError(w, http.StatusLocked, "Wallet is busy, try again later")
		return
	}
	defer s.walletLocks.Delete(lockKey)

	_, privateKeyHex, err := s.keyStore.LoadKey(intent.WalletID, intent.Chain, 0, s.password)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", err.Error())
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load key: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.lockTimeout)
	defer cancel()

	var txHash string
	tokenType := intent.TokenType
	if tokenType == "" {
		tokenType = "native"
	}

	switch {
	case tokenType == "spl":
		txHash, err = s.executeSPLIntent(ctx, intent, intentID, privateKeyHex)
	case tokenType == "erc20":
		txHash, err = s.executeERC20Intent(ctx, intent, intentID, privateKeyHex)
	default:
		txHash, err = s.executeNativeIntent(ctx, intent, intentID, privateKeyHex)
	}

	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to send tx: %v", err))
		return
	}

	s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusSent, txHash, "")

	log.Printf("Executed intent %s, tx hash: %s (token: %s)", intentID, txHash, tokenType)
	sendJSON(w, http.StatusOK, map[string]string{
		"tx_hash":   txHash,
		"intent_id": intentID,
	})
}

func (s *Server) executeNativeIntent(ctx context.Context, intent *wallet.TransactionIntent, intentID, privateKeyHex string) (string, error) {
	if intent.Chain == "solana" {
		return s.executeSolanaNative(ctx, intent, intentID, privateKeyHex)
	}
	return s.executeEthereumNative(ctx, intent, intentID, privateKeyHex)
}

func (s *Server) executeSolanaNative(ctx context.Context, intent *wallet.TransactionIntent, intentID, privateKeyHex string) (string, error) {
	lamports, err := solana.ParseSOL(intent.Value)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", "Invalid SOL value")
		return "", err
	}

	sufficient, _, err := s.txService.CheckBalanceSufficient(ctx, intent.Chain, intent.From, big.NewInt(int64(lamports)), 0)
	if err != nil {
		return "", err
	}
	if !sufficient {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", "Insufficient balance")
		return "", fmt.Errorf("insufficient balance")
	}

	if err := s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusExecuting, "", ""); err != nil {
		return "", err
	}

	solanaSigner, err := signer.NewSolanaSignerFromHex(privateKeyHex)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", err.Error())
		return "", err
	}
	defer solanaSigner.Zeroize()

	txHash, err := s.txService.SendSolanaTransaction(ctx, intent.Chain, intent.From, intent.To, lamports, solanaSigner, true)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", err.Error())
		return "", err
	}

	return txHash, nil
}

func (s *Server) executeEthereumNative(ctx context.Context, intent *wallet.TransactionIntent, intentID, privateKeyHex string) (string, error) {
	ethValue := new(big.Float)
	ethValue.SetString(intent.Value)
	weiValue := new(big.Int)
	ethValue.Mul(ethValue, new(big.Float).SetInt(big.NewInt(1e18)))
	ethValue.Int(weiValue)

	sufficient, required, err := s.txService.CheckBalanceSufficient(ctx, intent.Chain, intent.From, weiValue, intent.GasLimit)
	if err != nil {
		log.Printf("Warning: Balance check failed: %v", err)
	} else if !sufficient {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", fmt.Sprintf("Insufficient balance: %s wei", required.String()))
		return "", fmt.Errorf("insufficient balance")
	}

	if err := s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusExecuting, "", ""); err != nil {
		return "", err
	}

	ecdsaSigner, err := signer.NewECDSASignerFromHex(privateKeyHex)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", err.Error())
		return "", err
	}
	defer ecdsaSigner.Zeroize()

	feeSpeed := transaction.FeeSpeedStandard
	if intent.FeeSpeed == "slow" {
		feeSpeed = transaction.FeeSpeedSlow
	} else if intent.FeeSpeed == "fast" {
		feeSpeed = transaction.FeeSpeedFast
	}

	return s.txService.SendTransactionEIP1559(ctx, intent.Chain, intent.To, weiValue, ecdsaSigner, feeSpeed)
}

func (s *Server) executeERC20Intent(ctx context.Context, intent *wallet.TransactionIntent, intentID, privateKeyHex string) (string, error) {
	if intent.TokenAddress == "" {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", "Token address required for ERC-20")
		return "", fmt.Errorf("token address required")
	}

	tokenAmount := new(big.Float)
	tokenAmount.SetString(intent.Value)
	decimals := big.NewInt(1)
	for i := 0; i < int(intent.TokenDecimals); i++ {
		decimals.Mul(decimals, big.NewInt(10))
	}
	amount := new(big.Int)
	tokenAmount.Mul(tokenAmount, new(big.Float).SetInt(decimals))
	tokenAmount.Int(amount)

	sufficient, tokenBal, ethBal, err := s.txService.CheckTokenBalanceSufficient(ctx, intent.Chain, intent.TokenAddress, intent.From, amount, intent.GasLimit)
	if err != nil {
		log.Printf("Warning: Token balance check failed: %v", err)
	} else if !sufficient {
		msg := fmt.Sprintf("Insufficient token balance: %s", tokenBal.String())
		if ethBal != nil && ethBal.Cmp(big.NewInt(0)) == 0 {
			msg += " (also need ETH for gas)"
		}
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", msg)
		return "", fmt.Errorf("insufficient balance")
	}

	if err := s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusExecuting, "", ""); err != nil {
		return "", err
	}

	ecdsaSigner, err := signer.NewECDSASignerFromHex(privateKeyHex)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", err.Error())
		return "", err
	}
	defer ecdsaSigner.Zeroize()

	feeSpeed := transaction.FeeSpeedStandard
	if intent.FeeSpeed == "slow" {
		feeSpeed = transaction.FeeSpeedSlow
	} else if intent.FeeSpeed == "fast" {
		feeSpeed = transaction.FeeSpeedFast
	}

	return s.txService.SendERC20Transaction(ctx, intent.Chain, intent.TokenAddress, intent.To, amount, ecdsaSigner, feeSpeed)
}

func (s *Server) executeSPLIntent(ctx context.Context, intent *wallet.TransactionIntent, intentID, privateKeyHex string) (string, error) {
	if intent.TokenAddress == "" {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", "Token mint required for SPL")
		return "", fmt.Errorf("token mint required")
	}

	amount := new(big.Float)
	amount.SetString(intent.Value)
	tokenAmount := new(big.Int)
	decimals := big.NewInt(1)
	for i := 0; i < int(intent.TokenDecimals); i++ {
		decimals.Mul(decimals, big.NewInt(10))
	}
	amount.Mul(amount, new(big.Float).SetInt(decimals))
	amount.Int(tokenAmount)

	sufficient, _, _, err := s.txService.CheckTokenBalanceSufficient(ctx, intent.Chain, intent.TokenAddress, intent.From, tokenAmount, intent.GasLimit)
	if err != nil {
		log.Printf("Warning: SPL balance check failed: %v", err)
	} else if !sufficient {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", "Insufficient SPL token balance")
		return "", fmt.Errorf("insufficient balance")
	}

	if err := s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusExecuting, "", ""); err != nil {
		return "", err
	}

	solanaSigner, err := signer.NewSolanaSignerFromHex(privateKeyHex)
	if err != nil {
		s.intentStore.UpdateIntentStatus(intentID, wallet.IntentStatusFailed, "", err.Error())
		return "", err
	}
	defer solanaSigner.Zeroize()

	return s.txService.SendSPLTransaction(ctx, intent.Chain, intent.TokenAddress, intent.To, tokenAmount.Uint64(), solanaSigner, true)
}

func (s *Server) handleRetryIntent(w http.ResponseWriter, r *http.Request) {
	intentID := chi.URLParam(r, "id")

	intent, err := s.intentStore.GetIntent(intentID)
	if err != nil {
		sendError(w, http.StatusNotFound, fmt.Sprintf("Intent not found: %v", err))
		return
	}

	// Only failed or permanent_fail can be retried
	if intent.Status != wallet.IntentStatusFailed && intent.Status != wallet.IntentStatusPermanentFail {
		sendError(w, http.StatusBadRequest, fmt.Sprintf("Cannot retry intent in status: %s", intent.Status))
		return
	}

	// Check max retries
	if intent.RetryCount >= intent.MaxRetries {
		sendError(w, http.StatusBadRequest, fmt.Sprintf("Max retries (%d) already exceeded", intent.MaxRetries))
		return
	}

	// Retry the intent
	updatedIntent, err := s.intentStore.RetryIntent(intentID)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to retry intent: %v", err))
		return
	}

	log.Printf("Retried intent %s (attempt %d/%d)", intentID, updatedIntent.RetryCount, updatedIntent.MaxRetries)
	sendJSON(w, http.StatusOK, updatedIntent)
}

func (s *Server) runExpiryJob() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		count, err := s.intentStore.ExpireIntents()
		if err != nil {
			log.Printf("Expiry job error: %v", err)
		} else if count > 0 {
			log.Printf("Expired %d intents", count)
		}
	}
}

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleFeeEstimate(w http.ResponseWriter, r *http.Request) {
	chain := r.URL.Query().Get("chain")
	speed := r.URL.Query().Get("speed")

	if chain == "" {
		chain = "ethereum"
	}
	if speed == "" {
		speed = "standard"
	}

	// Get builder for chain
	builder, ok := s.txService.GetBuilder(chain)
	if !ok {
		sendError(w, http.StatusNotFound, "Chain not supported")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	feeInfo, err := builder.GetFeeInfo(ctx)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get fee estimate: %v", err))
		return
	}

	// Convert to gwei for readability
	toGwei := func(wei *big.Int) string {
		if wei == nil {
			return "0"
		}
		gwei := new(big.Float).Quo(new(big.Float).SetInt(wei), new(big.Float).SetInt(big.NewInt(1e9)))
		return gwei.Text('f', 2)
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"chain":            chain,
		"type":             "eip1559",
		"base_fee":         toGwei(feeInfo.BaseFee),
		"priority_fee":     toGwei(feeInfo.MaxPriorityFee),
		"max_fee":          toGwei(feeInfo.MaxFee),
		"gas_price_legacy": toGwei(feeInfo.LegacyGasPrice),
		"presets": map[string]string{
			"slow":     toGwei(feeInfo.Slow),
			"standard": toGwei(feeInfo.Standard),
			"fast":     toGwei(feeInfo.Fast),
		},
	})
}

func sendError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
