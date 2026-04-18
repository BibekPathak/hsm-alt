package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/yourorg/hsm/pkg/config"
	mpcnode "github.com/yourorg/hsm/pkg/mpc/node"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	flagConfigPath   = flag.String("config", "", "Path to configuration file")
	flagNodeID       = flag.Uint("node-id", 0, "Node ID")
	flagClusterID    = flag.String("cluster-id", "", "Cluster ID")
	flagThreshold    = flag.Uint("threshold", 2, "Threshold (t)")
	flagTotalNodes   = flag.Uint("total-nodes", 2, "Total nodes (n)")
	flagListenAddr   = flag.String("listen", ":7001", "Listen address for node-to-node gRPC")
	flagEnclavePort  = flag.Uint("enclave-port", 7002, "Enclave HTTP port")
	flagPeers        = flag.String("peers", "", "Comma-separated list of peers (e.g., 2:localhost:7011)")
	flagPasswordFile = flag.String("password-file", "", "Path to file containing MPC share password")
	flagShareDir     = flag.String("share-dir", "", "Directory for storing encrypted shares (default: ~/.hsm/mpc)")
)

func main() {
	flag.Parse()

	logger := setupLogger()
	defer logger.Sync()

	password := getPassword()
	if password == "" {
		fmt.Fprintf(os.Stderr, "ERROR: MPC_SHARE_PASSWORD environment variable or --password-file is required\n")
		fmt.Fprintf(os.Stderr, "Usage: Set MPC_SHARE_PASSWORD env var or use --password-file=/path/to/secret\n")
		os.Exit(1)
	}

	shareDir := getShareDir()
	fmt.Printf("MPC Share directory: %s\n", shareDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerAddrs := parsePeers(*flagPeers)

	nodeConfig := &config.NodeConfig{
		NodeID:      uint32(*flagNodeID),
		ClusterID:   *flagClusterID,
		Threshold:   uint32(*flagThreshold),
		TotalNodes:  uint32(*flagTotalNodes),
		ListenAddr:  *flagListenAddr,
		EnclaveAddr: fmt.Sprintf("localhost:%d", *flagEnclavePort),
		PeerAddrs:   peerAddrs,
		ShareFile:   shareDir + fmt.Sprintf("/node_%d/share.json", *flagNodeID),
	}

	if err := nodeConfig.Validate(); err != nil {
		logger.Fatal("Invalid node configuration", zap.Error(err))
	}

	logger.Info("Starting MPC Node",
		zap.Uint32("node_id", nodeConfig.NodeID),
		zap.String("cluster_id", nodeConfig.ClusterID),
		zap.Uint32("threshold", nodeConfig.Threshold),
		zap.Uint32("total_nodes", nodeConfig.TotalNodes),
		zap.String("share_file", nodeConfig.ShareFile),
	)

	shareStore := mpcnode.NewShareStore(shareDir)

	if shareStore.ShareExists(uint32(*flagNodeID)) {
		logger.Info("Loading existing key share from disk",
			zap.String("path", nodeConfig.ShareFile))

		clusterID, _, pubkey, err := shareStore.LoadShare(uint32(*flagNodeID), password)
		if err != nil {
			logger.Warn("Failed to load existing share, will participate in DKG",
				zap.Error(err))
		} else {
			logger.Info("Loaded existing key share",
				zap.String("cluster_id", clusterID),
				zap.Binary("public_key", pubkey))
		}
	} else {
		logger.Info("No existing key share found, will participate in DKG to generate one")
	}

	node, err := mpcnode.NewNode(nodeConfig, logger)
	if err != nil {
		logger.Fatal("Failed to create node", zap.Error(err))
	}

	if err := node.Start(ctx); err != nil {
		logger.Fatal("Failed to start node", zap.Error(err))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Shutting down due to signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
	}

	node.Stop()
	logger.Info("Node stopped")
}

func getPassword() string {
	if *flagPasswordFile != "" {
		data, err := os.ReadFile(*flagPasswordFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to read password file: %v\n", err)
			return ""
		}
		return strings.TrimSpace(string(data))
	}

	return os.Getenv("MPC_SHARE_PASSWORD")
}

func getShareDir() string {
	if *flagShareDir != "" {
		return *flagShareDir
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.hsm/mpc"
	}

	return homeDir + "/.hsm/mpc"
}

func parsePeers(peersStr string) map[uint32]string {
	peerAddrs := make(map[uint32]string)
	if peersStr == "" {
		return peerAddrs
	}

	for _, peer := range strings.Split(peersStr, ",") {
		parts := strings.Split(peer, ":")
		if len(parts) != 3 {
			fmt.Fprintf(os.Stderr, "Invalid peer format: %s (expected nodeID:host:port)\n", peer)
			continue
		}
		nodeID, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid peer node ID: %s\n", parts[0])
			continue
		}
		peerAddrs[uint32(nodeID)] = parts[1] + ":" + parts[2]
	}

	return peerAddrs
}

func setupLogger() *zap.Logger {
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}

	logger, _ := config.Build()
	return logger
}