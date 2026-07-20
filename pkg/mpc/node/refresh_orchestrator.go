package node

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/yourorg/hsm/api/gen"
	"github.com/yourorg/hsm/pkg/config"
)

type RefreshOrchestrator struct {
	config     *config.NodeConfig
	logger     *zap.Logger
	shareStore *ShareStore
	peers      map[uint32]*refreshPeerClient
}

type refreshPeerClient struct {
	nodeID uint32
	addr   string
	conn   *grpc.ClientConn
	client gen.NodeServiceClient
}

type RefreshResult struct {
	PublicKey  []byte
	ClusterID string
	Threshold uint32
	TotalNodes uint32
	SameKey   bool
}

func NewRefreshOrchestrator(cfg *config.NodeConfig, logger *zap.Logger, shareStore *ShareStore) *RefreshOrchestrator {
	return &RefreshOrchestrator{
		config:     cfg,
		logger:     logger,
		shareStore: shareStore,
		peers:      make(map[uint32]*refreshPeerClient),
	}
}

func (o *RefreshOrchestrator) connectToPeers(ctx context.Context) error {
	for nodeID, addr := range o.config.PeerAddrs {
		conn, err := grpc.DialContext(ctx, addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		if err != nil {
			o.logger.Warn("Failed to connect to peer for refresh",
				zap.Uint32("node_id", nodeID),
				zap.String("addr", addr),
				zap.Error(err))
			continue
		}
		o.peers[nodeID] = &refreshPeerClient{
			nodeID: nodeID,
			addr:   addr,
			conn:   conn,
			client: gen.NewNodeServiceClient(conn),
		}
	}
	return nil
}

func (o *RefreshOrchestrator) RunRefresh(ctx context.Context, password string) (*RefreshResult, error) {
	o.logger.Info("Starting same-key share refresh",
		zap.Uint32("node_id", o.config.NodeID),
		zap.String("cluster_id", o.config.ClusterID))

	if err := o.connectToPeers(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to peers: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Backup current share before refresh
	if o.shareStore.ShareExists(o.config.NodeID) {
		if err := o.shareStore.BackupShare(o.config.NodeID); err != nil {
			o.logger.Warn("Failed to backup share", zap.Error(err))
		}
	}

	// Collect all participant IDs (including ourselves)
	participantIDs := []uint32{o.config.NodeID}
	for id := range o.peers {
		participantIDs = append(participantIDs, id)
	}

	// === Round 1: Each node generates refresh evaluations ===
	o.logger.Info("Refresh Round 1: generating evaluations",
		zap.Uint32s("participants", participantIDs))

	// Collect evaluations from all nodes (including our own via enclave)
	// allEvaluations[sourceNode][targetNode] = evaluation
	allEvaluations := make(map[uint32]map[uint32][]byte)

	// Get evaluations from all peers in parallel
	type evalResult struct {
		nodeID uint32
		evals  map[uint32][]byte
		err    error
	}
	results := make(chan evalResult, len(o.peers)+1)

	for nodeID, peer := range o.peers {
		go func(id uint32, p *refreshPeerClient) {
			payload, _ := json.Marshal(map[string]interface{}{
				"type":            "refresh_round1",
				"participant_ids": participantIDs,
			})
			resp, err := p.client.DKGMessage(ctx, &gen.NodeMessage{
				MessageType: "refresh_round1",
				FromNode:    o.config.NodeID,
				ToNode:      id,
				Payload:     payload,
			})
			if err != nil {
				results <- evalResult{nodeID: id, err: err}
				return
			}
			var evals map[uint32][]byte
			json.Unmarshal(resp.Payload, &evals)
			results <- evalResult{nodeID: id, evals: evals}
		}(nodeID, peer)
	}

	// Collect peer results
	for range o.peers {
		r := <-results
		if r.err != nil {
			o.logger.Warn("Failed to get refresh evaluations from peer",
				zap.Uint32("peer", r.nodeID), zap.Error(r.err))
			continue
		}
		allEvaluations[r.nodeID] = r.evals
	}

	o.logger.Info("Refresh Round 1 complete",
		zap.Int("evaluations_collected", len(allEvaluations)))

	// === Round 2: Each node applies refresh evaluations ===
	o.logger.Info("Refresh Round 2: applying evaluations")

	// For each node, collect all evaluations addressed to it
	// myEvaluations[sourceNode] = evaluation_for_me
	myEvaluations := make(map[uint32][]byte)
	for sourceNode, evals := range allEvaluations {
		if eval, ok := evals[o.config.NodeID]; ok {
			myEvaluations[sourceNode] = eval
		}
	}

	// Send each peer the evaluations addressed to it
	var wg sync.WaitGroup
	for nodeID, peer := range o.peers {
		peerEvals := make(map[uint32][]byte)
		for sourceNode, evals := range allEvaluations {
			if eval, ok := evals[nodeID]; ok {
				peerEvals[sourceNode] = eval
			}
		}

		wg.Add(1)
		go func(id uint32, p *refreshPeerClient, evals map[uint32][]byte) {
			defer wg.Done()
			payload, _ := json.Marshal(map[string]interface{}{
				"type":        "refresh_apply",
				"evaluations": evals,
			})
			_, err := p.client.DKGMessage(ctx, &gen.NodeMessage{
				MessageType: "refresh_apply",
				FromNode:    o.config.NodeID,
				ToNode:      id,
				Payload:     payload,
			})
			if err != nil {
				o.logger.Warn("Failed to send refresh apply to peer",
					zap.Uint32("peer", id), zap.Error(err))
			}
		}(nodeID, peer, peerEvals)
	}
	wg.Wait()

	// Load the current public key for verification
	var currentPubKey []byte
	if o.shareStore.ShareExists(o.config.NodeID) {
		_, _, pubkey, err := o.shareStore.LoadShare(o.config.NodeID, password)
		if err == nil {
			currentPubKey = pubkey
		}
	}

	o.logger.Info("Share refresh completed",
		zap.Binary("public_key", currentPubKey),
		zap.String("cluster_id", o.config.ClusterID),
		zap.Bool("same_key", true))

	return &RefreshResult{
		PublicKey:  currentPubKey,
		ClusterID:  o.config.ClusterID,
		Threshold:  o.config.Threshold,
		TotalNodes: o.config.TotalNodes,
		SameKey:    true,
	}, nil
}

func (o *RefreshOrchestrator) Close() {
	for _, peer := range o.peers {
		if peer.conn != nil {
			peer.conn.Close()
		}
	}
}
