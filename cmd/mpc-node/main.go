package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourorg/hsm/pkg/config"
	mpcnode "github.com/yourorg/hsm/pkg/mpc/node"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	flagConfigPath = flag.String("config", "", "Path to configuration file")
	flagNodeID     = flag.Uint("node-id", 0, "Node ID")
	flagClusterID  = flag.String("cluster-id", "", "Cluster ID")
	flagThreshold  = flag.Uint("threshold", 3, "Threshold (t)")
	flagTotalNodes = flag.Uint("total-nodes", 5, "Total nodes (n)")
	flagListenAddr = flag.String("listen", ":7001", "Listen address for node-to-node gRPC")
)

func main() {
	flag.Parse()

	logger := setupLogger()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeConfig := &config.NodeConfig{
		NodeID:      uint32(*flagNodeID),
		ClusterID:   *flagClusterID,
		Threshold:   uint32(*flagThreshold),
		TotalNodes:  uint32(*flagTotalNodes),
		ListenAddr:  *flagListenAddr,
		EnclaveAddr: "localhost:7002",
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
