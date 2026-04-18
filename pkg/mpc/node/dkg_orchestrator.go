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

const (
	dkgTimeout      = 60 * time.Second
	peerConnTimeout = 10 * time.Second
)

type DKGOrchestrator struct {
	config     *config.NodeConfig
	logger     *zap.Logger
	peers      map[uint32]*dkgPeerClient
	shareStore *ShareStore
}

type dkgPeerClient struct {
	nodeID uint32
	addr   string
	conn   *grpc.ClientConn
	client gen.NodeServiceClient
}

type DKGResult struct {
	PublicKey   []byte
	KeyShares   map[uint32][]byte
	ClusterID   string
	Threshold   uint32
	TotalNodes  uint32
}

func NewDKGOrchestrator(cfg *config.NodeConfig, logger *zap.Logger, shareStore *ShareStore) *DKGOrchestrator {
	return &DKGOrchestrator{
		config:     cfg,
		logger:     logger,
		peers:      make(map[uint32]*dkgPeerClient),
		shareStore: shareStore,
	}
}

func (o *DKGOrchestrator) ConnectToPeers(ctx context.Context) error {
	o.logger.Info("Connecting to MPC peers",
		zap.Int("num_peers", len(o.config.PeerAddrs)))

	for nodeID, addr := range o.config.PeerAddrs {
		conn, err := grpc.DialContext(ctx, addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock())
		if err != nil {
			o.logger.Warn("Failed to connect to peer",
				zap.Uint32("node_id", nodeID),
				zap.String("addr", addr),
				zap.Error(err))
			continue
		}

		o.peers[nodeID] = &dkgPeerClient{
			nodeID: nodeID,
			addr:   addr,
			conn:   conn,
			client: gen.NewNodeServiceClient(conn),
		}
		o.logger.Info("Connected to peer",
			zap.Uint32("node_id", nodeID),
			zap.String("addr", addr))
	}

	return nil
}

func (o *DKGOrchestrator) Close() {
	for _, peer := range o.peers {
		if peer.conn != nil {
			peer.conn.Close()
		}
	}
}

func (o *DKGOrchestrator) RunDKG(ctx context.Context) (*DKGResult, error) {
	if err := o.ConnectToPeers(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to peers: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, dkgTimeout)
	defer cancel()

	clusterID := o.config.ClusterID
	if clusterID == "" {
		clusterID = fmt.Sprintf("cluster-%d-%d", time.Now().Unix())
	}

	threshold := o.config.Threshold
	totalNodes := o.config.TotalNodes

	if len(o.peers) < int(totalNodes)-1 {
		return nil, fmt.Errorf("not enough peers connected: have %d, need %d", len(o.peers), totalNodes-1)
	}

	round1Results, err := o.executeDKGRound1(ctx, clusterID, threshold, totalNodes)
	if err != nil {
		return nil, fmt.Errorf("DKG round 1 failed: %w", err)
	}

	round2Results, err := o.executeDKGRound2(ctx, clusterID, round1Results)
	if err != nil {
		return nil, fmt.Errorf("DKG round 2 failed: %w", err)
	}

	finalResult, err := o.executeDKGComplete(ctx, clusterID, round2Results)
	if err != nil {
		return nil, fmt.Errorf("DKG complete failed: %w", err)
	}

	result := &DKGResult{
		PublicKey:  finalResult.PublicKey,
		KeyShares:  make(map[uint32][]byte),
		ClusterID:  clusterID,
		Threshold:  threshold,
		TotalNodes: totalNodes,
	}

	result.KeyShares[o.config.NodeID] = finalResult.KeyShare

	o.logger.Info("DKG completed successfully",
		zap.String("cluster_id", clusterID),
		zap.Binary("public_key", finalResult.PublicKey),
		zap.Int("key_shares_collected", len(result.KeyShares)))

	return result, nil
}

type DKGMessagePayload struct {
	Type       string `json:"type"`
	Round      uint32 `json:"round"`
	FromNode   uint32 `json:"from_node"`
	ClusterID  string `json:"cluster_id"`
	Threshold  uint32 `json:"threshold"`
	TotalNodes uint32 `json:"total_nodes"`
	Data       []byte `json:"data"`
}

func (o *DKGOrchestrator) executeDKGRound1(ctx context.Context, clusterID string, threshold, totalNodes uint32) (map[uint32][]byte, error) {
	results := make(map[uint32][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup

	payload := DKGMessagePayload{
		Type:       "dkg_round1",
		Round:      1,
		FromNode:   o.config.NodeID,
		ClusterID:  clusterID,
		Threshold:  threshold,
		TotalNodes: totalNodes,
	}
	payloadBytes, _ := json.Marshal(payload)

	for nodeID, peer := range o.peers {
		wg.Add(1)
		go func(id uint32, p *dkgPeerClient) {
			defer wg.Done()

			resp, err := p.client.DKGMessage(ctx, &gen.NodeMessage{
				MessageType: "dkg_round1",
				FromNode:    o.config.NodeID,
				ToNode:      id,
				Payload:     payloadBytes,
				Timestamp:   uint64(time.Now().Unix()),
			})
			if err != nil {
				o.logger.Error("DKG Round 1 failed for peer",
					zap.Uint32("node_id", id),
					zap.Error(err))
				return
			}

			mu.Lock()
			results[id] = resp.Payload
			mu.Unlock()
		}(nodeID, peer)
	}

	wg.Wait()

	if len(results) < int(threshold)-1 {
		return nil, fmt.Errorf("not enough DKG round 1 responses: got %d, need %d", len(results), threshold-1)
	}

	return results, nil
}

func (o *DKGOrchestrator) executeDKGRound2(ctx context.Context, clusterID string, round1Results map[uint32][]byte) (map[uint32][]byte, error) {
	results := make(map[uint32][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup

	allMessages, _ := json.Marshal(round1Results)

	payload := DKGMessagePayload{
		Type:      "dkg_round2",
		Round:     2,
		FromNode:  o.config.NodeID,
		ClusterID: clusterID,
		Data:      allMessages,
	}
	payloadBytes, _ := json.Marshal(payload)

	for nodeID, peer := range o.peers {
		wg.Add(1)
		go func(id uint32, p *dkgPeerClient) {
			defer wg.Done()

			resp, err := p.client.DKGMessage(ctx, &gen.NodeMessage{
				MessageType: "dkg_round2",
				FromNode:    o.config.NodeID,
				ToNode:      id,
				Payload:     payloadBytes,
				Timestamp:   uint64(time.Now().Unix()),
			})
			if err != nil {
				o.logger.Error("DKG Round 2 failed for peer",
					zap.Uint32("node_id", id),
					zap.Error(err))
				return
			}

			mu.Lock()
			results[id] = resp.Payload
			mu.Unlock()
		}(nodeID, peer)
	}

	wg.Wait()

	if len(results) < int(o.config.Threshold)-1 {
		return nil, fmt.Errorf("not enough DKG round 2 responses")
	}

	return results, nil
}

func (o *DKGOrchestrator) executeDKGComplete(ctx context.Context, clusterID string, round2Results map[uint32][]byte) (*DKGCompleteResult, error) {
	type result struct {
		publicKey []byte
		keyShare  []byte
		err      error
	}

	resultCh := make(chan result, 1)

	go func() {
		for nodeID, peer := range o.peers {
			payload := DKGMessagePayload{
				Type:      "dkg_complete",
				Round:     3,
				FromNode:  o.config.NodeID,
				ClusterID: clusterID,
			}
			payloadBytes, _ := json.Marshal(payload)

			resp, err := peer.client.DKGMessage(ctx, &gen.NodeMessage{
				MessageType: "dkg_complete",
				FromNode:    o.config.NodeID,
				ToNode:      nodeID,
				Payload:     payloadBytes,
				Timestamp:   uint64(time.Now().Unix()),
			})
			if err != nil {
				resultCh <- result{err: err}
				return
			}

			var msgPayload DKGMessagePayload
			json.Unmarshal(resp.Payload, &msgPayload)

			resultCh <- result{
				publicKey: msgPayload.Data,
				keyShare:  msgPayload.Data,
			}
			return
		}
		resultCh <- result{err: fmt.Errorf("no peers available")}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-resultCh:
		if r.err != nil {
			return nil, r.err
		}
		return &DKGCompleteResult{
			PublicKey: r.publicKey,
			KeyShare:  r.keyShare,
		}, nil
	}
}

type DKGRound1Response struct {
	NodeID          uint32
	Share          []byte
	HostCommitment []byte
}

type DKGRound2Response struct {
	NodeID    uint32
	PublicKey []byte
}

type DKGCompleteResult struct {
	PublicKey []byte
	KeyShare  []byte
}

func (o *DKGOrchestrator) SaveShareToDisk(nodeID uint32, clusterID string, share, publicKey []byte, password string) error {
	if o.shareStore == nil {
		return fmt.Errorf("share store not configured")
	}

	return o.shareStore.SaveShare(nodeID, clusterID, share, publicKey, password)
}