package enclave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	addr   string
	client *http.Client
}

type InitRequest struct {
	ClusterID string `json:"cluster_id"`
}

type InitResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type StatusResponse struct {
	State       string `json:"state"`
	Epoch       uint32 `json:"epoch"`
	PublicKey   []byte `json:"public_key"`
	Initialized bool   `json:"initialized"`
}

type DkgStartRequest struct {
	MinSigners uint32 `json:"min_signers"`
	MaxSigners uint32 `json:"max_signers"`
}

type DkgStartResponse struct {
	Success    bool   `json:"success"`
	Round      uint32 `json:"round"`
	Round1Data []byte `json:"round1_data"`
}

type DKGPart1Request struct {
	SessionID  string `json:"session_id"`
	MinSigners uint32 `json:"min_signers"`
	MaxSigners uint32 `json:"max_signers"`
}

type DKGPart1Response struct {
	Success       bool   `json:"success"`
	Error         string `json:"error"`
	SecretPackage []byte `json:"secret_package"`
	Round1Package []byte `json:"round1_package"`
}

type DKGPart2Request struct {
	SessionID      string            `json:"session_id"`
	Round1Packages map[uint32][]byte `json:"round1_packages"`
}

type DKGPart2Response struct {
	Success        bool              `json:"success"`
	Error          string            `json:"error"`
	SecretPackage  []byte            `json:"secret_package"`
	Round2Packages map[uint32][]byte `json:"round2_packages"`
}

type DKGPart3Request struct {
	SessionID      string            `json:"session_id"`
	Round1Packages map[uint32][]byte `json:"round1_packages"`
	Round2Packages map[uint32][]byte `json:"round2_packages"`
}

type DKGPart3Response struct {
	Success       bool   `json:"success"`
	Error         string `json:"error"`
	KeyPackage    []byte `json:"key_package"`
	PubkeyPackage []byte `json:"pubkey_package"`
}

type SignRound1Request struct {
	SessionID string `json:"session_id"`
}

type SignStartRequest struct {
	SessionID    string   `json:"session_id"`
	Message      []byte   `json:"message"`
	Participants []uint32 `json:"participants"`
}

type SignStartResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type SignRound1Response struct {
	Success         bool   `json:"success"`
	Error           string `json:"error"`
	NonceCommitment []byte `json:"nonce_commitment"`
	Commitment      []byte `json:"commitment"`
}

type SignRound2Request struct {
	SessionID      string `json:"session_id"`
	SigningPackage []byte `json:"signing_package"`
}

type SignRound2Response struct {
	Success          bool   `json:"success"`
	Error            string `json:"error"`
	PartialSignature []byte `json:"partial_signature"`
	Commitment       []byte `json:"commitment"`
}

type PublicKeyResponse struct {
	PublicKey []byte `json:"public_key"`
}

type AggregateRequest struct {
	Message           []byte            `json:"message"`
	PartialSignatures map[uint32][]byte `json:"partial_signatures"`
}

type AggregateResponse struct {
	Success   bool   `json:"success"`
	Error     string `json:"error"`
	Signature []byte `json:"signature"`
}

func NewClient(addr string) (*Client, error) {
	return &Client{
		addr:   addr,
		client: &http.Client{},
	}, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	var body []byte
	if reqBody != nil {
		var err error
		body, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, "http://"+c.addr+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if reqBody != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func (c *Client) Initialize(ctx context.Context, clusterID string, threshold, totalShares uint32) error {
	req := InitRequest{ClusterID: clusterID}
	resp := &InitResponse{}

	if err := c.doRequest(ctx, "POST", "/initialize", req, resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("initialization failed: %s", resp.Error)
	}
	return nil
}

func (c *Client) GetStatus(ctx context.Context) (string, uint32, []byte, bool, error) {
	resp := &StatusResponse{}

	if err := c.doRequest(ctx, "GET", "/status", nil, resp); err != nil {
		return "", 0, nil, false, err
	}

	return resp.State, resp.Epoch, resp.PublicKey, resp.Initialized, nil
}

func (c *Client) StartDKG(ctx context.Context, minSigners, maxSigners uint32) error {
	req := DkgStartRequest{
		MinSigners: minSigners,
		MaxSigners: maxSigners,
	}
	resp := &DkgStartResponse{}

	if err := c.doRequest(ctx, "POST", "/dkg/start", req, resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("DKG failed")
	}
	return nil
}

func (c *Client) DKGPart1(ctx context.Context, sessionID string, minSigners, maxSigners uint32) ([]byte, []byte, error) {
	req := DKGPart1Request{
		SessionID:  sessionID,
		MinSigners: minSigners,
		MaxSigners: maxSigners,
	}
	resp := &DKGPart1Response{}

	if err := c.doRequest(ctx, "POST", "/dkg/part1", req, resp); err != nil {
		return nil, nil, err
	}

	if !resp.Success {
		return nil, nil, fmt.Errorf("DKG part1 failed: %s", resp.Error)
	}

	return resp.SecretPackage, resp.Round1Package, nil
}

func (c *Client) DKGPart2(ctx context.Context, sessionID string, secretPackage []byte, round1Packages map[uint32][]byte) ([]byte, map[uint32][]byte, error) {
	req := DKGPart2Request{
		SessionID:      sessionID,
		Round1Packages: round1Packages,
	}
	resp := &DKGPart2Response{}

	if err := c.doRequest(ctx, "POST", "/dkg/part2", req, resp); err != nil {
		return nil, nil, err
	}

	if !resp.Success {
		return nil, nil, fmt.Errorf("DKG part2 failed: %s", resp.Error)
	}

	return resp.SecretPackage, resp.Round2Packages, nil
}

func (c *Client) DKGPart3(ctx context.Context, sessionID string, secretPackage []byte, round1Packages, round2Packages map[uint32][]byte) ([]byte, []byte, error) {
	req := DKGPart3Request{
		SessionID:      sessionID,
		Round1Packages: round1Packages,
		Round2Packages: round2Packages,
	}
	resp := &DKGPart3Response{}

	if err := c.doRequest(ctx, "POST", "/dkg/part3", req, resp); err != nil {
		return nil, nil, err
	}

	if !resp.Success {
		return nil, nil, fmt.Errorf("DKG part3 failed: %s", resp.Error)
	}

	return resp.KeyPackage, resp.PubkeyPackage, nil
}

func (c *Client) GetPublicKey(ctx context.Context) ([]byte, error) {
	resp := &PublicKeyResponse{}

	if err := c.doRequest(ctx, "GET", "/public-key", nil, resp); err != nil {
		return nil, err
	}

	return resp.PublicKey, nil
}

func (c *Client) SignStart(ctx context.Context, sessionID string, message []byte, participants []uint32) error {
	req := SignStartRequest{
		SessionID:    sessionID,
		Message:      message,
		Participants: participants,
	}
	resp := &SignStartResponse{}

	if err := c.doRequest(ctx, "POST", "/sign/start", req, resp); err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("sign start failed: %s", resp.Error)
	}

	return nil
}

func (c *Client) SignRound1(ctx context.Context, sessionID string) ([]byte, []byte, error) {
	req := SignRound1Request{SessionID: sessionID}
	resp := &SignRound1Response{}

	if err := c.doRequest(ctx, "POST", "/sign/round1", req, resp); err != nil {
		return nil, nil, err
	}

	if !resp.Success {
		return nil, nil, fmt.Errorf("sign round1 failed: %s", resp.Error)
	}

	return resp.NonceCommitment, resp.Commitment, nil
}

func (c *Client) SignRound2(ctx context.Context, sessionID string, signingPackage []byte) ([]byte, []byte, error) {
	req := SignRound2Request{SessionID: sessionID, SigningPackage: signingPackage}
	resp := &SignRound2Response{}

	if err := c.doRequest(ctx, "POST", "/sign/round2", req, resp); err != nil {
		return nil, nil, err
	}

	if !resp.Success {
		return nil, nil, fmt.Errorf("sign round2 failed: %s", resp.Error)
	}

	return resp.PartialSignature, resp.Commitment, nil
}

func (c *Client) GetPubkeyPackage(ctx context.Context) ([]byte, error) {
	resp := &PublicKeyResponse{}
	if err := c.doRequest(ctx, "GET", "/public-key", nil, resp); err != nil {
		return nil, err
	}
	return resp.PublicKey, nil
}

func (c *Client) AggregateSignatures(ctx context.Context, message []byte, partialSignatures map[uint32][]byte) ([]byte, error) {
	req := AggregateRequest{
		Message:           message,
		PartialSignatures: partialSignatures,
	}
	resp := &AggregateResponse{}

	if err := c.doRequest(ctx, "POST", "/aggregate", req, resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("aggregate failed: %s", resp.Error)
	}

	return resp.Signature, nil
}

func (c *Client) VerifySignature(ctx context.Context, signature, message []byte) (bool, error) {
	type VerifyRequest struct {
		Signature []byte `json:"signature"`
		Message   []byte `json:"message"`
	}
	type VerifyResponse struct {
		Valid bool   `json:"valid"`
		Error string `json:"error"`
	}

	req := VerifyRequest{Signature: signature, Message: message}
	resp := &VerifyResponse{}

	if err := c.doRequest(ctx, "POST", "/verify", req, resp); err != nil {
		return false, err
	}

	return resp.Valid, nil
}

func (c *Client) Sign(ctx context.Context, message []byte, signers []uint32) ([]byte, error) {
	return nil, nil
}

func (c *Client) EvolveKey(ctx context.Context) (uint32, []byte, []byte, error) {
	return 1, []byte{}, []byte{}, nil
}

func (c *Client) GetAttestation(ctx context.Context) ([]byte, []byte, error) {
	return []byte{}, []byte{}, nil
}

func (c *Client) Close() error {
	return nil
}
