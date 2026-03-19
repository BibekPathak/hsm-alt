package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourorg/hsm/pkg/config"
	"github.com/yourorg/hsm/pkg/enclave"
	"github.com/yourorg/hsm/pkg/mpc/protocol"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	dkgSession  *protocol.DKGSession
	signSession *protocol.SigningSession

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

type Peer struct {
	NodeID   uint32
	Endpoint string
	conn     *grpc.ClientConn
	client   protocol.MPCNodeServiceClient
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

	n.connectToPeers()

	n.setState(StateReady)

	n.startHeartbeat()

	go n.runMessageLoop()

	return nil
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
	for nodeID, addr := range n.config.PeerAddrs {
		if uint32(nodeID) == n.config.NodeID {
			continue
		}

		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			n.logger.Warn("Failed to connect to peer", zap.Uint32("node_id", uint32(nodeID)), zap.Error(err))
			continue
		}

		client := protocol.NewMPCNodeServiceClient(conn)

		n.peers[uint32(nodeID)] = &Peer{
			NodeID:   uint32(nodeID),
			Endpoint: addr,
			conn:     conn,
			client:   client,
		}
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
		_, err := peer.client.Heartbeat(ctx, &protocol.HeartbeatRequest{
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
	n.logger.Info("Starting DKG as coordinator")

	minSigners := uint32(n.config.Threshold)
	maxSigners := uint32(n.config.TotalNodes)

	if err := n.enclave.StartDKG(ctx, minSigners, maxSigners); err != nil {
		return fmt.Errorf("DKG start failed: %w", err)
	}

	n.logger.Info("DKG completed successfully")
	return nil
}

func (n *MPCNode) Sign(ctx context.Context, message []byte, signers []uint32) ([]byte, error) {
	if n.getState() != StateReady {
		return nil, fmt.Errorf("node not ready, state: %v", n.getState())
	}

	n.setState(StateSigning)
	defer n.setState(StateReady)

	nonceCommit, _, err := n.enclave.SignRound1(ctx)
	if err != nil {
		return nil, fmt.Errorf("sign round1 failed: %w", err)
	}

	n.logger.Info("Round 1 complete",
		zap.Uint32("node_id", n.config.NodeID),
		zap.Binary("nonce_commitment", nonceCommit))

	partialSig, _, err := n.enclave.SignRound2(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("sign round2 failed: %w", err)
	}

	return partialSig, nil
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
