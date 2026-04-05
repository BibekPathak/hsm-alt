package protocol

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MPCNodeServiceClient interface {
	Handshake(ctx context.Context, in *HandshakeRequest, opts ...grpc.CallOption) (*HandshakeResponse, error)
	DKGRound1(ctx context.Context, in *DKGMessage, opts ...grpc.CallOption) (*DKGMessage, error)
	DKGRound2(ctx context.Context, in *DKGMessage, opts ...grpc.CallOption) (*DKGMessage, error)
	SigningRound1(ctx context.Context, in *SignMessage, opts ...grpc.CallOption) (*SignMessage, error)
	SigningRound2(ctx context.Context, in *SignMessage, opts ...grpc.CallOption) (*SignMessage, error)
	ResharingRound(ctx context.Context, in *ResharingMessage, opts ...grpc.CallOption) (*ResharingMessage, error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
}

type HandshakeRequest struct {
	NodeId      uint32
	ClusterId   string
	Attestation []byte
}

type HandshakeResponse struct {
	Accepted  bool
	NodeId    uint32
	ClusterId string
}

type DKGMessage struct {
	Round       uint32
	FromNode    uint32
	ToNode      uint32
	Message     []byte
	Commitments [][]byte
	Timestamp   uint64
}

type SignMessage struct {
	Round            uint32
	FromNode         uint32
	ToNode           uint32
	Message          []byte
	PartialSignature []byte
	Signers          []uint32
	Timestamp        uint64
}

type ResharingMessage struct {
	FromNode      uint32
	ToNode        uint32
	OldShare      []byte
	NewCommitment []byte
	Timestamp     uint64
}

type HeartbeatRequest struct {
	NodeId   uint32
	Sequence uint64
}

type HeartbeatResponse struct {
	NodeId   uint32
	Sequence uint64
	Healthy  bool
}

func NewMPCNodeServiceClient(conn *grpc.ClientConn) MPCNodeServiceClient {
	return nil
}

type DKGSession struct {
	Threshold  uint32
	TotalNodes uint32
	Round      uint32
}

func NewDKGSession(threshold, totalNodes uint32) *DKGSession {
	return &DKGSession{
		Threshold:  threshold,
		TotalNodes: totalNodes,
		Round:      0,
	}
}

func (s *DKGSession) Start() error {
	if s.Round != 0 {
		return fmt.Errorf("session already started")
	}
	s.Round = 1
	return nil
}

func (s *DKGSession) ProcessRound1(msg *DKGMessage) error {
	s.Round = 2
	return nil
}

func (s *DKGSession) ProcessRound2(msg *DKGMessage) error {
	s.Round = 3
	return nil
}

func (s *DKGSession) IsComplete() bool {
	return s.Round >= 3
}

type SigningSession struct {
	Message   []byte
	Signers   []uint32
	Threshold uint32
	Round     uint32
}

func NewSigningSession(message []byte, signers []uint32, threshold uint32) *SigningSession {
	return &SigningSession{
		Message:   message,
		Signers:   signers,
		Threshold: threshold,
		Round:     0,
	}
}

func (s *SigningSession) Start() error {
	s.Round = 1
	return nil
}

func (s *SigningSession) AddPartialSignature(sig []byte) error {
	return nil
}

func (s *SigningSession) IsComplete() bool {
	return s.Round >= 2
}

type NodeClient struct {
	nodeID   uint32
	endpoint string
	conn     *grpc.ClientConn
}

func NewNodeClient(nodeID uint32, endpoint string) (*NodeClient, error) {
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &NodeClient{
		nodeID:   nodeID,
		endpoint: endpoint,
		conn:     conn,
	}, nil
}

func (c *NodeClient) SendDKGMessage(ctx context.Context, msg *DKGMessage) (*DKGMessage, error) {
	return nil, nil
}

func (c *NodeClient) SendSignMessage(ctx context.Context, msg *SignMessage) (*SignMessage, error) {
	return nil, nil
}

func (c *NodeClient) HealthCheck(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return true, nil
}

func (c *NodeClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
