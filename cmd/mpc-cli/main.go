package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/yourorg/hsm/pkg/config"
	mpcnode "github.com/yourorg/hsm/pkg/mpc/node"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	flagPeers       = flag.String("peers", "", "Comma-separated peer addresses (localhost:8001,localhost:8002)")
	flagThreshold   = flag.Uint("threshold", 2, "Threshold (t)")
	flagTotalNodes  = flag.Uint("total-nodes", 3, "Total nodes (n)")
	flagClusterID   = flag.String("cluster-id", "", "Cluster ID")
	flagMessage     = flag.String("message", "", "Message to sign (base64 or hex)")
	flagNodeID      = flag.Uint("node-id", 1, "This node's ID")
	flagEnclaveAddr = flag.String("enclave", "localhost:7002", "Enclave HTTP address")
)

type CLICommand struct {
	Name        string
	Description string
	Run         func(args []string) error
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "dkg":
		runDKGCommand(args)
	case "sign":
		runSignCommand(args)
	case "key":
		runKeyCommand(args)
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("MPC CLI - Multi-Party Computation Operations")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  mpc-cli dkg [flags]     - Run Distributed Key Generation")
	fmt.Println("  mpc-cli sign [flags]    - Sign a message using MPC")
	fmt.Println("  mpc-cli key [flags]     - Key management operations")
	fmt.Println("  mpc-cli help            - Show this help message")
	fmt.Println("")
	fmt.Println("DKG Commands:")
	fmt.Println("  mpc-cli dkg init --cluster-id <id> --peers <addrs>")
	fmt.Println("    Initialize and run DKG to generate shared key")
	fmt.Println("")
	fmt.Println("Sign Commands:")
	fmt.Println("  mpc-cli sign --message <base64_or_hex>")
	fmt.Println("    Sign a message using MPC threshold signatures")
	fmt.Println("")
	fmt.Println("Key Commands:")
	fmt.Println("  mpc-cli key status")
	fmt.Println("    Check key share status")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  MPC_SHARE_PASSWORD=secret mpc-cli dkg init \\")
	fmt.Println("    --cluster-id wallet_123 \\")
	fmt.Println("    --peers localhost:8001,localhost:8002,localhost:8003")
	fmt.Println("")
	fmt.Println("  MPC_SHARE_PASSWORD=secret mpc-cli sign \\")
	fmt.Println("    --message base64EncodedMessage")
}

func runDKGCommand(args []string) {
	dkgCmd := flag.NewFlagSet("dkg", flag.ExitOnError)
	dkgCmd.StringVar(flagClusterID, "cluster-id", "", "Cluster ID")
	dkgCmd.StringVar(flagPeers, "peers", "", "Peer addresses (comma-separated)")
	dkgCmd.UintVar(flagThreshold, "threshold", 2, "Threshold")
	dkgCmd.UintVar(flagTotalNodes, "total-nodes", 3, "Total nodes")
	dkgCmd.UintVar(flagNodeID, "node-id", 1, "This node's ID")

	if len(args) == 0 {
		fmt.Println("Usage: mpc-cli dkg init [flags]")
		dkgCmd.PrintDefaults()
		os.Exit(1)
	}

	subCommand := args[0]
	if subCommand == "help" {
		fmt.Println("DKG Commands:")
		fmt.Println("  mpc-cli dkg init   - Initialize and run DKG")
		os.Exit(0)
	}

	if subCommand == "init" {
		dkgCmd.Parse(args[1:])
		runDKGInit()
	} else {
		fmt.Printf("Unknown dkg subcommand: %s\n", subCommand)
		os.Exit(1)
	}
}

func runDKGInit() {
	logger := setupLogger()

	password := os.Getenv("MPC_SHARE_PASSWORD")
	if password == "" {
		fmt.Fprintf(os.Stderr, "ERROR: MPC_SHARE_PASSWORD environment variable is required\n")
		os.Exit(1)
	}

	clusterID := *flagClusterID
	if clusterID == "" {
		clusterID = fmt.Sprintf("cluster-%d", time.Now().Unix())
		fmt.Printf("No cluster-id specified, using: %s\n", clusterID)
	}

	peerAddrs := parsePeers(*flagPeers)
	if len(peerAddrs) < int(*flagTotalNodes)-1 {
		fmt.Fprintf(os.Stderr, "ERROR: Not enough peers. Need %d, got %d\n", *flagTotalNodes-1, len(peerAddrs))
		os.Exit(1)
	}

	cfg := &config.NodeConfig{
		NodeID:      uint32(*flagNodeID),
		ClusterID:   clusterID,
		Threshold:   uint32(*flagThreshold),
		TotalNodes:  uint32(*flagTotalNodes),
		ListenAddr:  fmt.Sprintf("localhost:%d", 8000+*flagNodeID),
		EnclaveAddr: *flagEnclaveAddr,
		PeerAddrs:   peerAddrs,
	}

	logger.Info("Starting DKG",
		zap.String("cluster_id", clusterID),
		zap.Uint32("threshold", cfg.Threshold),
		zap.Uint32("total_nodes", cfg.TotalNodes))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	shareStore := mpcnode.NewShareStore(getShareDir())
	orchestrator := mpcnode.NewDKGOrchestrator(cfg, logger, shareStore)

	result, err := orchestrator.RunDKG(ctx)
	if err != nil {
		logger.Error("DKG failed", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("\n✅ DKG Completed Successfully!\n")
	fmt.Printf("   Cluster ID:   %s\n", result.ClusterID)
	fmt.Printf("   Public Key:   %x\n", result.PublicKey)
	fmt.Printf("   Threshold:   %d-of-%d\n", result.Threshold, result.TotalNodes)
	fmt.Printf("   Key Shares:   %d nodes\n", len(result.KeyShares))

	if err := orchestrator.SaveShareToDisk(cfg.NodeID, result.ClusterID, result.KeyShares[cfg.NodeID], result.PublicKey, password); err != nil {
		logger.Error("Failed to save share to disk", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("   Share saved:  %s/node_%d/share.json\n", getShareDir(), cfg.NodeID)

	orchestrator.Close()
}

func runSignCommand(args []string) {
	signCmd := flag.NewFlagSet("sign", flag.ExitOnError)
	signCmd.StringVar(flagMessage, "message", "", "Message to sign (base64 or hex)")

	if len(args) == 0 {
		fmt.Println("Usage: mpc-cli sign --message <base64_or_hex>")
		signCmd.PrintDefaults()
		os.Exit(1)
	}

	signCmd.Parse(args)

	if *flagMessage == "" {
		fmt.Fprintf(os.Stderr, "ERROR: --message is required\n")
		os.Exit(1)
	}

	logger := setupLogger()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := &config.NodeConfig{
		NodeID:     uint32(*flagNodeID),
		Threshold: uint32(*flagThreshold),
		PeerAddrs:  parsePeers(*flagPeers),
	}

	orchestrator := mpcnode.NewSignOrchestrator(cfg, logger)
	defer orchestrator.Close()

	var msgBytes []byte
	if strings.HasPrefix(*flagMessage, "base64:") {
		msgBytes = mustBase64Decode(strings.TrimPrefix(*flagMessage, "base64:"))
	} else if strings.HasPrefix(*flagMessage, "hex:") {
		msgBytes = mustHexDecode(strings.TrimPrefix(*flagMessage, "hex:"))
	} else {
		msgBytes = []byte(*flagMessage)
	}

	result, err := orchestrator.SignMessage(ctx, msgBytes)
	if err != nil {
		logger.Error("Signing failed", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("\n✅ MPC Signing Completed!\n")
	fmt.Printf("   Session ID:   %s\n", result.SessionID)
	fmt.Printf("   Signers:      %v\n", result.Signers)
	fmt.Printf("   Signature:    %x\n", result.Signature)
}

func runKeyCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Key Commands:")
		fmt.Println("  mpc-cli key status   - Show key share status")
		os.Exit(0)
	}

	subCommand := args[0]
	switch subCommand {
	case "status":
		runKeyStatus()
	default:
		fmt.Printf("Unknown key subcommand: %s\n", subCommand)
		os.Exit(1)
	}
}

func runKeyStatus() {
	password := os.Getenv("MPC_SHARE_PASSWORD")
	if password == "" {
		fmt.Fprintf(os.Stderr, "ERROR: MPC_SHARE_PASSWORD is required\n")
		os.Exit(1)
	}

	shareStore := mpcnode.NewShareStore(getShareDir())

	nodeID := uint32(*flagNodeID)
	if shareStore.ShareExists(nodeID) {
		clusterID, share, pubkey, err := shareStore.LoadShare(nodeID, password)
		if err != nil {
			fmt.Printf("❌ Failed to load share: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Key Share Found\n")
		fmt.Printf("   Node ID:     node_%d\n", nodeID)
		fmt.Printf("   Cluster ID:  %s\n", clusterID)
		fmt.Printf("   Public Key:  %x\n", pubkey)
		fmt.Printf("   Share Size:  %d bytes\n", len(share))
	} else {
		fmt.Printf("❌ No key share found for node_%d\n", nodeID)
		fmt.Printf("   Run 'mpc-cli dkg init' to generate one\n")
		os.Exit(1)
	}
}

func parsePeers(peersStr string) map[uint32]string {
	peerAddrs := make(map[uint32]string)
	if peersStr == "" {
		return peerAddrs
	}

	addrs := strings.Split(peersStr, ",")
	for i, addr := range addrs {
		peerAddrs[uint32(i+1)] = strings.TrimSpace(addr)
	}
	return peerAddrs
}

func getShareDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.hsm/mpc"
	}
	return homeDir + "/.hsm/mpc"
}

func mustBase64Decode(s string) []byte {
	data, err := json.RawMessage(s).MarshalJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode base64: %v\n", err)
		os.Exit(1)
	}
	return data
}

func mustHexDecode(s string) []byte {
	fmt.Fprintf(os.Stderr, "hex decoding not implemented\n")
	os.Exit(1)
	return nil
}

func setupLogger() *zap.Logger {
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Encoding:         "console",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, _ := config.Build()
	return logger
}

func copyStringSlice(s []string) []string {
	result := make([]string, len(s))
	copy(result, s)
	return result
}

func printJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}