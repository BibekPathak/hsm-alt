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
	flagConfigPath  = flag.String("config", "", "Path to configuration file")
	flagNodeID      = flag.Uint("node-id", 0, "Node ID")
	flagClusterID   = flag.String("cluster-id", "", "Cluster ID")
	flagThreshold   = flag.Uint("threshold", 2, "Threshold (t)")
	flagTotalNodes  = flag.Uint("total-nodes", 2, "Total nodes (n)")
	flagListenAddr  = flag.String("listen", ":7001", "Listen address for node-to-node gRPC")
	flagEnclavePort = flag.Uint("enclave-port", 7002, "Enclave HTTP port")
	flagPeers       = flag.String("peers", "", "Comma-separated list of peers (e.g., 2:localhost:7011)")
)

func main() {
	flag.Parse()

	logger := setupLogger()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse peers (format: "nodeID:host:port,nodeID:host:port")
	// Example: "1:localhost:7001" or "2:127.0.0.1:7011"
	peerAddrs := make(map[uint32]string)
	if *flagPeers != "" {
		for _, peer := range strings.Split(*flagPeers, ",") {
			parts := strings.Split(peer, ":")
			if len(parts) != 3 {
				fmt.Fprintf(os.Stderr, "Invalid peer format: %s (expected nodeID:host:port)\n", peer)
				os.Exit(1)
			}
			nodeID, err := strconv.ParseUint(parts[0], 10, 32)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid peer node ID: %s\n", parts[0])
				os.Exit(1)
			}
			peerAddrs[uint32(nodeID)] = parts[1] + ":" + parts[2]
		}
	}

	nodeConfig := &config.NodeConfig{
		NodeID:      uint32(*flagNodeID),
		ClusterID:   *flagClusterID,
		Threshold:   uint32(*flagThreshold),
		TotalNodes:  uint32(*flagTotalNodes),
		ListenAddr:  *flagListenAddr,
		EnclaveAddr: fmt.Sprintf("localhost:%d", *flagEnclavePort),
		PeerAddrs:   peerAddrs,
	}

	logger.Info("Starting MPC Node",
		zap.Uint32("node_id", nodeConfig.NodeID),
		zap.String("cluster_id", nodeConfig.ClusterID),
		zap.Uint32("threshold", nodeConfig.Threshold),
		zap.Uint32("total_nodes", nodeConfig.TotalNodes),
	)

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
