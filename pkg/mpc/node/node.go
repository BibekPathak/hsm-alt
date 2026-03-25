package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/hsm/api/gen"
	"github.com/yourorg/hsm/pkg/config"
	"github.com/yourorg/hsm/pkg/enclave"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	signRound1Timeout = 10 * time.Second
	signRound2Timeout = 15 * time.Second
	aggregateTimeout  = 5 * time.Second
	peerCallTimeout   = 10 * time.Second
)

var (
	ErrNodeUnreachable = errors.New("node unreachable")
	ErrNodeTimeout     = errors.New("node timeout")
	ErrInvalidResponse = errors.New("invalid response from node")
	ErrInsufficientSig = errors.New("insufficient valid signatures")
)

type NodeState int

const (
	StateInitializing NodeState = iota
	StateKeyGeneration
	StateReady
	StateSigning
	StateResharing
	StateRecovering
	StateFailed
)

type MPCNode struct {
	config      *config.NodeConfig
	logger      *zap.Logger
	state       NodeState
	peers       map[uint32]*Peer
	enclave     *enclave.Client
	dkgSession  *dkgSessionInfo
	signSession *signingSessionInfo

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

type dkgSessionInfo struct {
	sessionID      string
	minSigners     uint32
	maxSigners     uint32
	secretPkg1     []byte
	secretPkg2     []byte
	round1Packages map[uint32][]byte
	round2Packages map[uint32][]byte
	startTime      uint64
	round          uint32
}

type Peer struct {
	NodeID   uint32
	Endpoint string
	conn     *grpc.ClientConn
	client   gen.NodeServiceClient
}

func (p *Peer) SendSignMessage(ctx context.Context, msgType string, payload []byte) (*gen.NodeMessage, error) {
	if p.client == nil {
		return nil, fmt.Errorf("no client connection")
	}
	return p.client.SignMessage(ctx, &gen.NodeMessage{
		MessageType: msgType,
		FromNode:    p.NodeID,
		Payload:     payload,
	})
}

func (p *Peer) callWithTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(ctx)
}

func (p *Peer) SendDKGMessage(ctx context.Context, msgType string, payload []byte) (*gen.NodeMessage, error) {
	if p.client == nil {
		return nil, fmt.Errorf("no client connection")
	}
	return p.client.DKGMessage(ctx, &gen.NodeMessage{
		MessageType: msgType,
		FromNode:    p.NodeID,
		Payload:     payload,
	})
}

func (p *Peer) TriggerSign(ctx context.Context, message []byte, signers []uint32, sessionID string) (string, error) {
	if p.client == nil {
		return "", fmt.Errorf("no client connection")
	}
	resp, err := p.client.TriggerSign(ctx, &gen.TriggerSignRequest{
		Message:   message,
		Signers:   signers,
		SessionId: sessionID,
	})
	if err != nil {
		return "", err
	}
	if !resp.Accepted {
		return "", fmt.Errorf("sign start rejected: %s", resp.Error)
	}
	return resp.SessionId, nil
}

func (p *Peer) Aggregate(ctx context.Context, message []byte, partialSigs map[uint32][]byte) ([]byte, error) {
	if p.client == nil {
		return nil, fmt.Errorf("no client connection")
	}
	resp, err := p.client.AggregateSignatures(ctx, &gen.AggregateRequest{
		Message:           message,
		PartialSignatures: partialSigs,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("aggregation failed: %s", resp.Error)
	}
	return resp.Signature, nil
}

func (p *Peer) DirectMessage(ctx context.Context, sessionID, msgType string, fromNode, toNode uint32, payload []byte) ([]byte, error) {
	if p.client == nil {
		return nil, fmt.Errorf("no client connection")
	}
	resp, err := p.client.DirectMessage(ctx, &gen.DirectMessageRequest{
		SessionId:   sessionID,
		MessageType: msgType,
		FromNode:    fromNode,
		ToNode:      toNode,
		Payload:     payload,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("direct message failed: %s", resp.Error)
	}
	return resp.Payload, nil
}

func NewNode(cfg *config.NodeConfig, logger *zap.Logger) (*MPCNode, error) {
	ctx, cancel := context.WithCancel(context.Background())

	enclaveClient, err := enclave.NewClient(cfg.EnclaveAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create enclave client: %w", err)
	}

	return &MPCNode{
		config:  cfg,
		logger:  logger,
		state:   StateInitializing,
		peers:   make(map[uint32]*Peer),
		enclave: enclaveClient,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func (n *MPCNode) Start(ctx context.Context) error {
	n.logger.Info("Starting MPC node", zap.Uint32("node_id", n.config.NodeID))

	if err := n.enclave.Initialize(ctx, n.config.ClusterID, n.config.Threshold, n.config.TotalNodes); err != nil {
		return fmt.Errorf("failed to initialize enclave: %w", err)
	}

	go n.startGRPCServer()

	n.connectToPeers()

	n.setState(StateReady)

	n.startHeartbeat()

	go n.runMessageLoop()

	return nil
}

func (n *MPCNode) startGRPCServer() {
	lis, err := net.Listen("tcp", n.config.ListenAddr)
	if err != nil {
		n.logger.Fatal("Failed to listen", zap.Error(err))
		return
	}

	grpcServer := grpc.NewServer()
	RegisterNodeServiceServer(grpcServer, n)

	n.logger.Info("gRPC server listening", zap.String("addr", n.config.ListenAddr))
	if err := grpcServer.Serve(lis); err != nil {
		n.logger.Error("gRPC server failed", zap.Error(err))
	}
}

func (n *MPCNode) Stop() {
	n.cancel()
	n.setState(StateInitializing)

	for _, peer := range n.peers {
		if peer.conn != nil {
			peer.conn.Close()
		}
	}
}

func (n *MPCNode) connectToPeers() {
	var attestation []byte
	if n.enclave != nil {
		ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
		quote, _, isSim, err := n.enclave.GetAttestation(ctx)
		cancel()
		if err != nil {
			n.logger.Warn("Failed to get local attestation", zap.Error(err))
		} else if isSim {
			attestation = quote
			n.logger.Info("Got local attestation for peer handshake", zap.Bool("simulation", isSim))
		}
	}

	for nodeID, addr := range n.config.PeerAddrs {
		if uint32(nodeID) == n.config.NodeID {
			continue
		}

		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			n.logger.Warn("Failed to connect to peer", zap.Uint32("node_id", uint32(nodeID)), zap.Error(err))
			continue
		}

		client := gen.NewNodeServiceClient(conn)

		peer := &Peer{
			NodeID:   uint32(nodeID),
			Endpoint: addr,
			conn:     conn,
			client:   client,
		}

		ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
		resp, err := client.Handshake(ctx, &gen.HandshakeRequest{
			NodeId:      n.config.NodeID,
			ClusterId:   n.config.ClusterID,
			Attestation: attestation,
		})
		cancel()

		if err != nil {
			n.logger.Warn("Handshake failed with peer", zap.Uint32("node_id", uint32(nodeID)), zap.Error(err))
			conn.Close()
			continue
		}

		if !resp.Accepted {
			n.logger.Warn("Peer rejected handshake", zap.Uint32("node_id", uint32(nodeID)))
			conn.Close()
			continue
		}

		n.logger.Info("Handshake successful with peer", zap.Uint32("node_id", uint32(nodeID)))
		n.peers[uint32(nodeID)] = peer
	}
}

func (n *MPCNode) startHeartbeat() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-n.ctx.Done():
				return
			case <-ticker.C:
				n.checkPeersHealth()
			}
		}
	}()
}

func (n *MPCNode) checkPeersHealth() {
	for _, peer := range n.peers {
		ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
		_, err := peer.client.Heartbeat(ctx, &gen.HeartbeatRequest{
			NodeId:   n.config.NodeID,
			Sequence: uint64(time.Now().Unix()),
		})
		cancel()

		if err != nil {
			n.logger.Warn("Peer unhealthy", zap.Uint32("node_id", peer.NodeID), zap.Error(err))
		}
	}
}

func (n *MPCNode) runMessageLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (n *MPCNode) GetStatus() (NodeState, uint32, []byte) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.state, 0, nil
}

func (n *MPCNode) RunDKG(ctx context.Context) error {
	threshold := uint32(n.config.Threshold)

	var lastErr error
	participants := make([]uint32, 0, len(n.peers)+1)
	participants = append(participants, n.config.NodeID)
	for nodeID := range n.peers {
		participants = append(participants, nodeID)
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if len(participants) < int(threshold) {
			return fmt.Errorf("not enough participants: have %d, need %d", len(participants), threshold)
		}

		n.logger.Info("DKG attempt",
			zap.Int("attempt", attempt+1),
			zap.Uint32s("participants", participants))

		err := n.doRunDKG(ctx, participants)
		if err == nil {
			return nil
		}

		lastErr = err
		n.logger.Warn("DKG attempt failed, will retry",
			zap.Int("attempt", attempt+1),
			zap.Error(err))

		// Remove failed participants and retry
		participants = n.excludeFailedParticipants(participants, err)
	}

	return fmt.Errorf("DKG failed after %d attempts: %w", maxRetries, lastErr)
}

func (n *MPCNode) excludeFailedParticipants(participants []uint32, err error) []uint32 {
	if len(participants) <= 1 {
		return participants
	}
	return participants[:len(participants)-1]
}

func (n *MPCNode) doRunDKG(ctx context.Context, participants []uint32) error {
	n.logger.Info("Starting DKG as coordinator")

	minSigners := uint32(n.config.Threshold)
	maxSigners := uint32(n.config.TotalNodes)
	sessionID := uuid.New().String()

	n.logger.Info("DKG participants", zap.Uint32s("participants", participants))

	// ===== ROUND 1: Collect round1 packages from all participants =====
	n.logger.Info("DKG Round 1: Collecting round1 packages", zap.String("session_id", sessionID))
	round1Packages := make(map[uint32][]byte)

	// Get our own round1 package
	ourSecretPkg1, round1Pkg, err := n.enclave.DKGPart1(ctx, sessionID, minSigners, maxSigners)
	if err != nil {
		return fmt.Errorf("DKG part1 failed: %w", err)
	}
	round1Packages[n.config.NodeID] = round1Pkg
	n.logger.Info("Generated round1 package", zap.Uint32("node_id", n.config.NodeID))

	// Collect round1 packages from all other participants
	for nodeID := range n.peers {
		peer := n.peers[nodeID]
		n.logger.Info("Requesting round1 from peer", zap.Uint32("peer_id", nodeID))

		// Send request to peer - peer will generate round1 and return it
		resp, err := peer.SendDKGMessage(ctx, "dkg_round1", []byte(sessionID))
		if err != nil {
			n.logger.Warn("Failed to get round1 from peer", zap.Uint32("peer_id", nodeID), zap.Error(err))
			continue
		}
		round1Packages[nodeID] = resp.Payload
	}

	if len(round1Packages) != len(participants) {
		n.logger.Warn("Not all participants responded in round1",
			zap.Int("received", len(round1Packages)),
			zap.Int("expected", len(participants)))
	}

	n.logger.Info("Round 1 complete", zap.Int("num_packages", len(round1Packages)))

	// ===== ROUND 2: Send round1 packages to all participants, collect round2 =====
	n.logger.Info("DKG Round 2: Processing round1 packages", zap.String("session_id", sessionID))

	// Encode round1 packages to send to peers (indexed by node ID)
	round1PackagesJSON, err := json.Marshal(round1Packages)
	if err != nil {
		return fmt.Errorf("failed to encode round1 packages: %w", err)
	}

	// Our round2 - pass secret package from our round1
	ourSecretPkg2, round2Packages, err := n.enclave.DKGPart2(ctx, sessionID, ourSecretPkg1, round1Packages)
	if err != nil {
		return fmt.Errorf("DKG part2 failed: %w", err)
	}

	allRound2Packages := make(map[uint32][]byte)
	// round2Packages is also indexed by node ID, get our entry
	if pkg, ok := round2Packages[n.config.NodeID]; ok {
		allRound2Packages[n.config.NodeID] = pkg
	}

	// Get round2 from peers
	for nodeID := range n.peers {
		peer := n.peers[nodeID]
		n.logger.Info("Requesting round2 from peer", zap.Uint32("peer_id", nodeID))

		resp, err := peer.SendDKGMessage(ctx, "dkg_round2", round1PackagesJSON)
		if err != nil {
			n.logger.Warn("Failed to get round2 from peer", zap.Uint32("peer_id", nodeID), zap.Error(err))
			continue
		}
		allRound2Packages[nodeID] = resp.Payload
	}

	n.logger.Info("Round 2 complete", zap.Int("num_packages", len(allRound2Packages)))

	// ===== ROUND 3: Complete DKG =====
	n.logger.Info("DKG Round 3: Completing DKG", zap.String("session_id", sessionID))

	// Encode both round1 and round2 packages for round3
	roundCompleteJSON, err := json.Marshal(map[string]interface{}{
		"round1": round1Packages,
		"round2": allRound2Packages,
	})
	if err != nil {
		return fmt.Errorf("failed to encode round packages: %w", err)
	}

	// Our final DKG
	keyPackage, pubkeyPackage, err := n.enclave.DKGPart3(ctx, sessionID, ourSecretPkg2, round1Packages, allRound2Packages)
	if err != nil {
		return fmt.Errorf("DKG part3 failed: %w", err)
	}
	_ = keyPackage

	// Get final DKG results from peers
	for nodeID := range n.peers {
		peer := n.peers[nodeID]
		n.logger.Info("Requesting round3 from peer", zap.Uint32("peer_id", nodeID))

		_, err := peer.SendDKGMessage(ctx, "dkg_round3", roundCompleteJSON)
		if err != nil {
			n.logger.Warn("Failed to get round3 from peer", zap.Uint32("peer_id", nodeID), zap.Error(err))
			continue
		}
	}

	n.logger.Info("DKG completed successfully",
		zap.Uint32("node_id", n.config.NodeID),
		zap.Binary("public_key", pubkeyPackage))

	return nil
}

const maxRetries = 3

func (n *MPCNode) Sign(ctx context.Context, message []byte, signers []uint32) ([]byte, error) {
	if n.getState() != StateReady {
		return nil, fmt.Errorf("node not ready, state: %v", n.getState())
	}

	threshold := uint32(n.config.Threshold)

	var lastErr error
	currentSigners := make([]uint32, len(signers))
	copy(currentSigners, signers)

	for attempt := 0; attempt < maxRetries; attempt++ {
		if len(currentSigners) < int(threshold) {
			return nil, fmt.Errorf("not enough signers: have %d, need %d", len(currentSigners), threshold)
		}

		n.logger.Info("Signing attempt",
			zap.Int("attempt", attempt+1),
			zap.Uint32s("signers", currentSigners))

		sig, err := n.doSign(ctx, message, currentSigners)
		if err == nil {
			return sig, nil
		}

		lastErr = err
		n.logger.Warn("Signing attempt failed, will retry",
			zap.Int("attempt", attempt+1),
			zap.Error(err))

		// Remove failed signers and retry
		currentSigners = n.excludeFailedSigners(currentSigners, err)
	}

	return nil, fmt.Errorf("signing failed after %d attempts: %w", maxRetries, lastErr)
}

func (n *MPCNode) excludeFailedSigners(signers []uint32, err error) []uint32 {
	if len(signers) <= 1 {
		return signers
	}

	errStr := err.Error()

	for _, signer := range signers {
		if signer == n.config.NodeID {
			continue
		}
		signerStr := fmt.Sprintf("peer %d", signer)
		if strings.Contains(errStr, signerStr) {
			n.logger.Info("Excluding failed peer", zap.Uint32("peer_id", signer))
			newSigners := make([]uint32, 0, len(signers)-1)
			for _, s := range signers {
				if s != signer {
					newSigners = append(newSigners, s)
				}
			}
			return newSigners
		}
	}

	_ = errStr
	return signers[:len(signers)-1]
}

func (n *MPCNode) doSign(ctx context.Context, message []byte, signers []uint32) ([]byte, error) {
	n.setState(StateSigning)
	defer n.setState(StateReady)

	sessionID := uuid.New().String()
	n.logger.Info("Starting signing session",
		zap.String("session_id", sessionID),
		zap.Binary("message", message),
		zap.Uint32s("signers", signers))

	// Step 1: Start signing session on all participants
	for _, nodeID := range signers {
		if nodeID == n.config.NodeID {
			if err := n.enclave.SignStart(ctx, sessionID, message, signers); err != nil {
				return nil, fmt.Errorf("sign start failed for node %d: %w", nodeID, err)
			}
		} else {
			peer, ok := n.peers[nodeID]
			if !ok {
				return nil, fmt.Errorf("peer %d not found", nodeID)
			}
			_, err := peer.SendSignMessage(ctx, "sign_start", []byte(sessionID))
			if err != nil {
				return nil, fmt.Errorf("sign start failed for peer %d: %w", nodeID, err)
			}
		}
	}

	// Step 2: Round 1 - Each participant generates round1 data
	// For direct communication, we collect directly from each participant
	commitments := make(map[uint32][]byte)
	for _, nodeID := range signers {
		if nodeID == n.config.NodeID {
			_, commitment, err := n.enclave.SignRound1(ctx, sessionID)
			if err != nil {
				return nil, fmt.Errorf("sign round1 failed: %w", err)
			}
			commitments[nodeID] = commitment
		} else {
			peer, ok := n.peers[nodeID]
			if !ok {
				return nil, fmt.Errorf("peer %d not found", nodeID)
			}
			// Use DirectMessage for direct node-to-node communication
			payload, err := peer.DirectMessage(ctx, sessionID, "round1_commitment", n.config.NodeID, nodeID, nil)
			if err != nil {
				return nil, fmt.Errorf("round1 failed for peer %d: %w", nodeID, err)
			}
			commitments[nodeID] = payload
		}
	}

	// Broadcast commitments to all signers (they need them for round2)
	// Each signer needs all commitments to compute their partial signature
	for _, nodeID := range signers {
		if nodeID == n.config.NodeID {
			continue
		}
		peer, ok := n.peers[nodeID]
		if !ok {
			continue
		}
		commitmentsJSON, _ := json.Marshal(commitments)
		_, err := peer.SendSignMessage(ctx, "commitments", commitmentsJSON)
		if err != nil {
			n.logger.Warn("Failed to send commitments to peer", zap.Uint32("peer", nodeID), zap.Error(err))
		}
	}

	n.logger.Info("Round 1 complete",
		zap.String("session_id", sessionID),
		zap.Int("num_commitments", len(commitments)))

	// Step 3: Round 2 - Get partial signatures from all participants using direct communication
	partialSignatures := make(map[uint32][]byte)
	for _, nodeID := range signers {
		if nodeID == n.config.NodeID {
			partialSig, _, err := n.enclave.SignRound2(ctx, sessionID, nil)
			if err != nil {
				return nil, fmt.Errorf("sign round2 failed: %w", err)
			}
			partialSignatures[nodeID] = partialSig
		} else {
			peer, ok := n.peers[nodeID]
			if !ok {
				return nil, fmt.Errorf("peer %d not found", nodeID)
			}
			// Use DirectMessage for direct node-to-node communication
			commitmentsJSON, _ := json.Marshal(commitments)
			payload, err := peer.DirectMessage(ctx, sessionID, "round2_partial", n.config.NodeID, nodeID, commitmentsJSON)
			if err != nil {
				return nil, fmt.Errorf("round2 failed for peer %d: %w", nodeID, err)
			}
			partialSignatures[nodeID] = payload
		}
	}

	// Need at least threshold signatures
	threshold := uint32(n.config.Threshold)
	if len(partialSignatures) < int(threshold) {
		return nil, fmt.Errorf("not enough partial signatures: got %d, need %d", len(partialSignatures), threshold)
	}

	n.logger.Info("Round 2 complete",
		zap.String("session_id", sessionID),
		zap.Int("num_partial_sigs", len(partialSignatures)))

	// Step 4: Aggregate partial signatures
	signature, err := n.enclave.AggregateSignatures(ctx, message, partialSignatures)
	if err != nil {
		return nil, fmt.Errorf("aggregate failed: %w", err)
	}

	n.logger.Info("Signing complete",
		zap.String("session_id", sessionID),
		zap.Binary("signature", signature))

	return signature, nil
}

func (n *MPCNode) StartDKG(ctx context.Context, minSigners, maxSigners uint32) error {
	n.setState(StateKeyGeneration)

	err := n.enclave.StartDKG(ctx, minSigners, maxSigners)
	if err != nil {
		n.setState(StateFailed)
		return err
	}

	return nil
}

func (n *MPCNode) EvolveKey(ctx context.Context) error {
	if n.getState() != StateReady {
		return fmt.Errorf("node not ready")
	}

	_, _, _, err := n.enclave.EvolveKey(ctx)
	return err
}

func (n *MPCNode) Reshare(ctx context.Context) error {
	if n.getState() != StateReady {
		return fmt.Errorf("node not ready, state: %v", n.getState())
	}

	n.logger.Info("Starting key resharing")

	// Resharing in FROST is essentially running DKG again to generate new shares
	// For true share refresh (same public key, new shares), a more complex protocol is needed
	// Here we implement it as DKG re-run which gives new key share to each participant
	return n.doRunDKG(ctx, nil)
}

func (n *MPCNode) setState(state NodeState) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.state = state
}

func (n *MPCNode) getState() NodeState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state
}
