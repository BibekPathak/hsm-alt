package node

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/hsm/api/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type MPCNodeServiceServer struct {
	gen.UnimplementedNodeServiceServer
	node *MPCNode
}

func RegisterNodeServiceServer(grpcServer *grpc.Server, node *MPCNode) {
	gen.RegisterNodeServiceServer(grpcServer, &MPCNodeServiceServer{
		node: node,
	})
}

func (s *MPCNodeServiceServer) Handshake(ctx context.Context, req *gen.HandshakeRequest) (*gen.HandshakeResponse, error) {
	s.node.logger.Info("Received handshake",
		zap.Uint32("from_node", req.NodeId),
		zap.String("cluster_id", req.ClusterId),
		zap.Int("attestation_len", len(req.Attestation)))

	if req.ClusterId != s.node.config.ClusterID {
		s.node.logger.Warn("Cluster ID mismatch",
			zap.String("expected", s.node.config.ClusterID),
			zap.String("received", req.ClusterId))
		return &gen.HandshakeResponse{
			Accepted:  false,
			NodeId:    s.node.config.NodeID,
			ClusterId: s.node.config.ClusterID,
		}, nil
	}

	verified := false
	if len(req.Attestation) > 0 {
		quote := req.Attestation
		if len(quote) > 16 && string(quote[:14]) == "SGX_SIMULATION_" {
			s.node.logger.Info("Accepted simulation mode attestation",
				zap.Uint32("from_node", req.NodeId))
			verified = true
		}
	} else {
		s.node.logger.Warn("No attestation provided", zap.Uint32("from_node", req.NodeId))
	}

	if !verified {
		s.node.logger.Warn("Attestation verification failed",
			zap.Uint32("from_node", req.NodeId))
		return &gen.HandshakeResponse{
			Accepted:  false,
			NodeId:    s.node.config.NodeID,
			ClusterId: s.node.config.ClusterID,
		}, nil
	}

	s.node.logger.Info("Handshake accepted",
		zap.Uint32("from_node", req.NodeId))

	return &gen.HandshakeResponse{
		Accepted:  true,
		NodeId:    s.node.config.NodeID,
		ClusterId: s.node.config.ClusterID,
	}, nil
}

func (s *MPCNodeServiceServer) DKGMessage(ctx context.Context, req *gen.NodeMessage) (*gen.NodeMessage, error) {
	s.node.logger.Info("Received DKG message",
		zap.String("type", req.MessageType),
		zap.Uint32("from_node", req.FromNode))

	var respPayload []byte
	var err error

	switch req.MessageType {
	case "trigger_dkg":
		err = s.handleTriggerDKG(ctx)
		respPayload = []byte{}
	case "trigger_reshare":
		err = s.handleTriggerReshare(ctx)
		respPayload = []byte{}
	case "dkg_round1":
		respPayload, err = s.handleDKGRound1(ctx, req)
	case "dkg_round2":
		respPayload, err = s.handleDKGRound2(ctx, req)
	case "dkg_round3":
		err = s.handleDKGRound3(ctx, req)
		respPayload = []byte{} // Round3 doesn't return anything
	default:
		err = fmt.Errorf("unknown DKG message type: %s", req.MessageType)
	}

	if err != nil {
		s.node.logger.Error("DKG message handling failed", zap.Error(err))
		return &gen.NodeMessage{
			MessageType: "error",
			FromNode:    s.node.config.NodeID,
			ToNode:      req.FromNode,
			Payload:     []byte(err.Error()),
		}, nil
	}

	return &gen.NodeMessage{
		MessageType: req.MessageType + "_response",
		FromNode:    s.node.config.NodeID,
		ToNode:      req.FromNode,
		Payload:     respPayload,
	}, nil
}

type dkgPhase1Data struct {
	Packages map[uint32][]byte `json:"packages"`
}

type dkgPhaseComplete struct {
	Round1 []byte            `json:"round1"`
	Round2 map[uint32][]byte `json:"round2"`
}

func (s *MPCNodeServiceServer) handleTriggerDKG(ctx context.Context) error {
	go func() {
		err := s.node.RunDKG(context.Background())
		if err != nil {
			s.node.logger.Error("DKG failed", zap.Error(err))
		}
	}()
	return nil
}

func (s *MPCNodeServiceServer) handleTriggerReshare(ctx context.Context) error {
	go func() {
		err := s.node.Reshare(context.Background())
		if err != nil {
			s.node.logger.Error("Reshare failed", zap.Error(err))
		} else {
			s.node.logger.Info("Reshare completed successfully")
		}
	}()
	return nil
}

func (s *MPCNodeServiceServer) handleDKGRound1(ctx context.Context, req *gen.NodeMessage) ([]byte, error) {
	sessionID := string(req.Payload)
	minSigners := uint32(s.node.config.Threshold)
	maxSigners := uint32(s.node.config.TotalNodes)

	// Store DKG session info
	s.node.mu.Lock()
	s.node.dkgSession = &dkgSessionInfo{
		sessionID:  sessionID,
		minSigners: minSigners,
		maxSigners: maxSigners,
		startTime:  uint64(time.Now().Unix()),
		round:      1,
	}
	s.node.mu.Unlock()

	secretPkg1, round1Pkg, err := s.node.enclave.DKGPart1(ctx, sessionID, minSigners, maxSigners)
	if err != nil {
		return nil, fmt.Errorf("DKG part1 failed: %w", err)
	}

	// Store secret package for later rounds
	s.node.mu.Lock()
	if s.node.dkgSession != nil {
		s.node.dkgSession.secretPkg1 = secretPkg1
	}
	s.node.mu.Unlock()

	return round1Pkg, nil
}

func (s *MPCNodeServiceServer) handleDKGRound2(ctx context.Context, req *gen.NodeMessage) ([]byte, error) {
	var round1Packages map[uint32][]byte
	if err := json.Unmarshal(req.Payload, &round1Packages); err != nil {
		return nil, fmt.Errorf("failed to parse round1 packages: %w", err)
	}

	s.node.mu.Lock()
	sessionID := s.node.dkgSession.sessionID
	secretPkg1 := s.node.dkgSession.secretPkg1
	s.node.mu.Unlock()

	_, round2Pkg, err := s.node.enclave.DKGPart2(ctx, sessionID, secretPkg1, round1Packages)
	if err != nil {
		return nil, fmt.Errorf("DKG part2 failed: %w", err)
	}

	// Store secret package for round3
	s.node.mu.Lock()
	if s.node.dkgSession != nil {
		for _, pkg := range round2Pkg {
			s.node.dkgSession.secretPkg2 = pkg
			break
		}
	}
	s.node.mu.Unlock()

	// Return just our round2 package
	for _, pkg := range round2Pkg {
		return pkg, nil
	}

	return nil, fmt.Errorf("no round2 package generated")
}

func (s *MPCNodeServiceServer) handleDKGRound3(ctx context.Context, req *gen.NodeMessage) error {
	var data map[string]interface{}
	if err := json.Unmarshal(req.Payload, &data); err != nil {
		return fmt.Errorf("failed to parse round data: %w", err)
	}

	round1JSON, ok := data["round1"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("round1 not found in payload")
	}
	round2JSON, ok := data["round2"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("round2 not found in payload")
	}

	// Convert interface{} to []byte
	round1Packages := make(map[uint32][]byte)
	for k, v := range round1JSON {
		var id uint32
		fmt.Sscanf(k, "%d", &id)
		round1Packages[id] = v.([]byte)
	}

	round2Packages := make(map[uint32][]byte)
	for k, v := range round2JSON {
		var id uint32
		fmt.Sscanf(k, "%d", &id)
		round2Packages[id] = v.([]byte)
	}

	s.node.mu.Lock()
	sessionID := s.node.dkgSession.sessionID
	secretPkg2 := s.node.dkgSession.secretPkg2
	s.node.mu.Unlock()

	_, _, err := s.node.enclave.DKGPart3(ctx, sessionID, secretPkg2, round1Packages, round2Packages)
	if err != nil {
		return fmt.Errorf("DKG part3 failed: %w", err)
	}

	return nil
}

func (s *MPCNodeServiceServer) SignMessage(ctx context.Context, req *gen.NodeMessage) (*gen.NodeMessage, error) {
	s.node.logger.Info("Received sign message",
		zap.String("type", req.MessageType),
		zap.Uint32("from_node", req.FromNode))

	var respPayload []byte
	var err error

	switch req.MessageType {
	case "trigger_sign":
		err, respPayload = s.handleTriggerSign(ctx, req)
	case "sign_start":
		err = s.handleSignStart(ctx, req)
	case "round1":
		respPayload, err = s.handleSignRound1(ctx, req)
	case "round2":
		respPayload, err = s.handleSignRound2(ctx, req)
	case "commitments":
		err = s.handleCommitments(ctx, req)
		respPayload = []byte{}
	default:
		err = fmt.Errorf("unknown message type: %s", req.MessageType)
	}

	if err != nil {
		s.node.logger.Error("Sign message handling failed", zap.Error(err))
		return &gen.NodeMessage{
			MessageType: "error",
			FromNode:    s.node.config.NodeID,
			ToNode:      req.FromNode,
			Payload:     []byte(err.Error()),
		}, nil
	}

	return &gen.NodeMessage{
		MessageType: req.MessageType + "_response",
		FromNode:    s.node.config.NodeID,
		ToNode:      req.FromNode,
		Payload:     respPayload,
	}, nil
}

func (s *MPCNodeServiceServer) handleTriggerSign(ctx context.Context, req *gen.NodeMessage) (error, []byte) {
	message := req.Payload
	signers := []uint32{s.node.config.NodeID}
	for id := range s.node.peers {
		signers = append(signers, id)
	}

	// Run signing synchronously to get the signature
	sig, err := s.node.Sign(context.Background(), message, signers)
	if err != nil {
		return err, []byte{}
	}

	return nil, sig
}

func (s *MPCNodeServiceServer) handleSignStart(ctx context.Context, req *gen.NodeMessage) error {
	s.node.mu.Lock()
	defer s.node.mu.Unlock()

	if s.node.signSession != nil {
		return fmt.Errorf("signing session already in progress")
	}

	s.node.signSession = &signingSessionInfo{
		sessionID:    string(req.Payload),
		message:      req.Payload,
		participants: nil,
		startTime:    uint64(time.Now().Unix()),
		round:        1,
	}

	return nil
}

func (s *MPCNodeServiceServer) handleSignRound1(ctx context.Context, req *gen.NodeMessage) ([]byte, error) {
	sessionID := string(req.Payload)
	nonceCommit, commitment, err := s.node.enclave.SignRound1(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	response := &SignRound1Payload{
		NonceCommitment: nonceCommit,
		Commitment:      commitment,
	}
	return json.Marshal(response)
}

func (s *MPCNodeServiceServer) handleSignRound2(ctx context.Context, req *gen.NodeMessage) ([]byte, error) {
	sessionID := string(req.Payload)
	partialSig, _, err := s.node.enclave.SignRound2(ctx, sessionID, nil)
	if err != nil {
		return nil, err
	}

	response := &SignRound2Payload{
		PartialSignature: partialSig,
	}
	return json.Marshal(response)
}

func (s *MPCNodeServiceServer) handleCommitments(ctx context.Context, req *gen.NodeMessage) error {
	var commitments map[uint32][]byte
	if err := json.Unmarshal(req.Payload, &commitments); err != nil {
		return fmt.Errorf("failed to unmarshal commitments: %w", err)
	}

	s.node.logger.Info("Received commitments",
		zap.Int("num_commitments", len(commitments)))

	return nil
}

func (s *MPCNodeServiceServer) Heartbeat(ctx context.Context, req *gen.HeartbeatRequest) (*gen.HeartbeatResponse, error) {
	return &gen.HeartbeatResponse{
		NodeId:   s.node.config.NodeID,
		Sequence: req.Sequence,
		Healthy:  true,
	}, nil
}

func (s *MPCNodeServiceServer) TriggerSign(ctx context.Context, req *gen.TriggerSignRequest) (*gen.TriggerSignResponse, error) {
	s.node.logger.Info("Received TriggerSign request",
		zap.Binary("message", req.Message),
		zap.Uint32s("signers", req.Signers),
		zap.String("session_id", req.SessionId))

	if s.node.enclave == nil {
		return &gen.TriggerSignResponse{
			Accepted:  false,
			SessionId: req.SessionId,
			Error:     "enclave not initialized",
		}, nil
	}

	sessionID := req.SessionId
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	err := s.node.enclave.SignStart(ctx, sessionID, req.Message, req.Signers)
	if err != nil {
		s.node.logger.Error("SignStart failed", zap.Error(err))
		return &gen.TriggerSignResponse{
			Accepted:  false,
			SessionId: sessionID,
			Error:     err.Error(),
		}, nil
	}

	s.node.logger.Info("Sign session started on node",
		zap.String("session_id", sessionID))

	return &gen.TriggerSignResponse{
		Accepted:  true,
		SessionId: sessionID,
		Error:     "",
	}, nil
}

func (s *MPCNodeServiceServer) AggregateSignatures(ctx context.Context, req *gen.AggregateRequest) (*gen.AggregateResponse, error) {
	s.node.logger.Info("Received AggregateSignatures request",
		zap.Binary("message", req.Message),
		zap.Int("num_partials", len(req.PartialSignatures)))

	if s.node.enclave == nil {
		return &gen.AggregateResponse{
			Success:   false,
			Signature: nil,
			Error:     "enclave not initialized",
		}, nil
	}

	sig, err := s.node.enclave.AggregateSignatures(ctx, req.Message, req.PartialSignatures)
	if err != nil {
		s.node.logger.Error("Aggregate failed", zap.Error(err))
		return &gen.AggregateResponse{
			Success:   false,
			Signature: nil,
			Error:     err.Error(),
		}, nil
	}

	s.node.logger.Info("Aggregation successful",
		zap.Binary("signature", sig))

	return &gen.AggregateResponse{
		Success:   true,
		Signature: sig,
		Error:     "",
	}, nil
}

func (s *MPCNodeServiceServer) DirectMessage(ctx context.Context, req *gen.DirectMessageRequest) (*gen.DirectMessageResponse, error) {
	s.node.logger.Info("Received DirectMessage",
		zap.String("session_id", req.SessionId),
		zap.String("message_type", req.MessageType),
		zap.Uint32("from_node", req.FromNode),
		zap.Uint32("to_node", req.ToNode),
		zap.Int("payload_size", len(req.Payload)))

	var respPayload []byte
	var err error

	switch req.MessageType {
	case "round1_commitment":
		err = s.node.enclave.SignStart(ctx, req.SessionId, nil, nil)
		if err != nil {
			return &gen.DirectMessageResponse{Success: false, Payload: nil, Error: err.Error()}, nil
		}
		_, commitment, err := s.node.enclave.SignRound1(ctx, req.SessionId)
		if err != nil {
			return &gen.DirectMessageResponse{Success: false, Payload: nil, Error: err.Error()}, nil
		}
		respPayload = commitment

	case "round2_partial":
		_, partialSig, err := s.node.enclave.SignRound2(ctx, req.SessionId, req.Payload)
		if err != nil {
			return &gen.DirectMessageResponse{Success: false, Payload: nil, Error: err.Error()}, nil
		}
		respPayload = partialSig

	default:
		s.node.logger.Warn("Unknown direct message type", zap.String("type", req.MessageType))
		return &gen.DirectMessageResponse{Success: false, Payload: nil, Error: "unknown message type"}, nil
	}

	s.node.logger.Info("DirectMessage processed",
		zap.String("session_id", req.SessionId),
		zap.String("message_type", req.MessageType))

	return &gen.DirectMessageResponse{
		Success: true,
		Payload: respPayload,
		Error:   "",
	}, nil
}

func (s *MPCNodeServiceServer) AbortSign(ctx context.Context, req *gen.AbortSignRequest) (*gen.AbortSignResponse, error) {
	s.node.logger.Info("Received AbortSign request",
		zap.String("session_id", req.SessionId))

	if s.node.enclave == nil {
		return &gen.AbortSignResponse{
			Success: false,
			Error:   "enclave not initialized",
		}, nil
	}

	err := s.node.enclave.SignAbort(ctx, req.SessionId)
	if err != nil {
		s.node.logger.Warn("SignAbort failed", zap.Error(err))
		return &gen.AbortSignResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	s.node.logger.Info("Sign session aborted",
		zap.String("session_id", req.SessionId))

	return &gen.AbortSignResponse{
		Success: true,
		Error:   "",
	}, nil
}

type signingSessionInfo struct {
	sessionID    string
	message      []byte
	participants []uint32
	startTime    uint64
	round        uint32
	mu           sync.Mutex
}

type SignRound1Payload struct {
	NonceCommitment []byte
	Commitment      []byte
}

type SignRound2Payload struct {
	PartialSignature []byte
}
