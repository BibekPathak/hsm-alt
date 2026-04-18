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
	signRoundTimeout = 15 * time.Second
	signAggrTimeout  = 10 * time.Second
)

type SignOrchestrator struct {
	config    *config.NodeConfig
	logger   *zap.Logger
	peers     map[uint32]*signPeerClient
	threshold uint32
}

type signPeerClient struct {
	nodeID uint32
	addr   string
	conn   *grpc.ClientConn
	client gen.NodeServiceClient
}

type SignResult struct {
	Signature []byte
	SessionID string
	Signers   []uint32
}

func NewSignOrchestrator(cfg *config.NodeConfig, logger *zap.Logger) *SignOrchestrator {
	return &SignOrchestrator{
		config:    cfg,
		logger:    logger,
		peers:     make(map[uint32]*signPeerClient),
		threshold: cfg.Threshold,
	}
}

func (o *SignOrchestrator) ConnectToPeers(ctx context.Context) error {
	o.logger.Info("Connecting to MPC peers for signing",
		zap.Int("num_peers", len(o.config.PeerAddrs)))

	for nodeID, addr := range o.config.PeerAddrs {
		conn, err := grpc.DialContext(ctx, addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(peerConnTimeout))
		if err != nil {
			o.logger.Warn("Failed to connect to peer for signing",
				zap.Uint32("node_id", nodeID),
				zap.String("addr", addr),
				zap.Error(err))
			continue
		}

		o.peers[nodeID] = &signPeerClient{
			nodeID: nodeID,
			addr:   addr,
			conn:   conn,
			client: gen.NewNodeServiceClient(conn),
		}
		o.logger.Info("Connected to peer for signing",
			zap.Uint32("node_id", nodeID),
			zap.String("addr", addr))
	}

	return nil
}

func (o *SignOrchestrator) Close() {
	for _, peer := range o.peers {
		if peer.conn != nil {
			peer.conn.Close()
		}
	}
}

type SignMessagePayload struct {
	Type      string `json:"type"`
	Round     uint32 `json:"round"`
	FromNode  uint32 `json:"from_node"`
	SessionID string `json:"session_id"`
	Message   []byte `json:"message"`
	Data      []byte `json:"data"`
}

func (o *SignOrchestrator) SignMessage(ctx context.Context, message []byte) (*SignResult, error) {
	if len(o.peers) == 0 {
		if err := o.ConnectToPeers(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to peers: %w", err)
		}
	}

	if uint32(len(o.peers)) < o.threshold {
		return nil, fmt.Errorf("not enough peers connected: have %d, need %d", len(o.peers), o.threshold)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sessionID := fmt.Sprintf("sign-%d-%d", time.Now().UnixNano(), o.config.NodeID)

	signers := o.selectSigners(int(o.threshold))
	o.logger.Info("Starting MPC signing",
		zap.String("session_id", sessionID),
		zap.Uint32s("signers", signers))

	round1Results, err := o.executeSignRound1(ctx, message, sessionID, signers)
	if err != nil {
		return nil, fmt.Errorf("sign round 1 failed: %w", err)
	}

	round2Results, err := o.executeSignRound2(ctx, sessionID, round1Results)
	if err != nil {
		return nil, fmt.Errorf("sign round 2 failed: %w", err)
	}

	finalSig, err := o.aggregateSignatures(ctx, message, sessionID, round2Results)
	if err != nil {
		return nil, fmt.Errorf("signature aggregation failed: %w", err)
	}

	return &SignResult{
		Signature: finalSig,
		SessionID: sessionID,
		Signers:   signers,
	}, nil
}

func (o *SignOrchestrator) selectSigners(threshold int) []uint32 {
	signers := make([]uint32, 0, threshold)
	for nodeID := range o.peers {
		signers = append(signers, nodeID)
		if len(signers) >= threshold {
			break
		}
	}
	return signers
}

func (o *SignOrchestrator) executeSignRound1(ctx context.Context, message []byte, sessionID string, signers []uint32) (map[uint32][]byte, error) {
	results := make(map[uint32][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup

	payload := SignMessagePayload{
		Type:      "sign_round1",
		Round:     1,
		FromNode:  o.config.NodeID,
		SessionID: sessionID,
		Message:   message,
	}
	payloadBytes, _ := json.Marshal(payload)

	for _, nodeID := range signers {
		peer, ok := o.peers[nodeID]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(id uint32, p *signPeerClient) {
			defer wg.Done()

			resp, err := p.client.SignMessage(ctx, &gen.NodeMessage{
				MessageType: "sign_round1",
				FromNode:    o.config.NodeID,
				ToNode:      id,
				Payload:     payloadBytes,
				Timestamp:   uint64(time.Now().Unix()),
			})
			if err != nil {
				o.logger.Error("Sign Round 1 failed for peer",
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

	if len(results) < int(o.threshold) {
		return nil, fmt.Errorf("not enough sign round 1 responses: got %d, need %d", len(results), o.threshold)
	}

	return results, nil
}

func (o *SignOrchestrator) executeSignRound2(ctx context.Context, sessionID string, round1Results map[uint32][]byte) (map[uint32][]byte, error) {
	results := make(map[uint32][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup

	signingPackage, _ := json.Marshal(round1Results)

	payload := SignMessagePayload{
		Type:      "sign_round2",
		Round:     2,
		FromNode:  o.config.NodeID,
		SessionID: sessionID,
		Data:      signingPackage,
	}
	payloadBytes, _ := json.Marshal(payload)

	for nodeID, peer := range o.peers {
		if _, ok := round1Results[nodeID]; !ok {
			continue
		}

		wg.Add(1)
		go func(id uint32, p *signPeerClient) {
			defer wg.Done()

			resp, err := p.client.SignMessage(ctx, &gen.NodeMessage{
				MessageType: "sign_round2",
				FromNode:    o.config.NodeID,
				ToNode:      id,
				Payload:     payloadBytes,
				Timestamp:   uint64(time.Now().Unix()),
			})
			if err != nil {
				o.logger.Error("Sign Round 2 failed for peer",
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

	if len(results) < int(o.threshold) {
		return nil, fmt.Errorf("not enough sign round 2 responses")
	}

	return results, nil
}

func (o *SignOrchestrator) aggregateSignatures(ctx context.Context, message []byte, sessionID string, partialSigs map[uint32][]byte) ([]byte, error) {
	results := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func() {
		for nodeID, peer := range o.peers {
			partials := make(map[uint32][]byte)
			for id, sig := range partialSigs {
				partials[id] = sig
			}
			partials[o.config.NodeID] = partialSigs[nodeID]

			resp, err := peer.client.AggregateSignatures(ctx, &gen.AggregateRequest{
				Message:           message,
				PartialSignatures: partials,
			})
			if err != nil {
				errChan <- err
				return
			}

			if resp.Success {
				results <- resp.Signature
				return
			}
		}
		errChan <- fmt.Errorf("all aggregate attempts failed")
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case sig := <-results:
		return sig, nil
	case err := <-errChan:
		return nil, err
	}
}

func (o *SignOrchestrator) AbortSign(ctx context.Context, sessionID string) error {
	var wg sync.WaitGroup

	for _, peer := range o.peers {
		wg.Add(1)
		go func(p *signPeerClient) {
			defer wg.Done()
			req := &gen.AbortSignRequest{SessionId: sessionID}
			p.client.AbortSign(ctx, req)
		}(peer)
	}

	wg.Wait()
	return nil
}