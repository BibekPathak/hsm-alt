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

type SignRound1Response struct {
	Success         bool   `json:"success"`
	NonceCommitment []byte `json:"nonce_commitment"`
	Commitment      []byte `json:"commitment"`
}

type SignRound2Request struct {
	SigningPackage []byte `json:"signing_package"`
}

type SignRound2Response struct {
	Success          bool   `json:"success"`
	PartialSignature []byte `json:"partial_signature"`
	Commitment       []byte `json:"commitment"`
}

type PublicKeyResponse struct {
	PublicKey []byte `json:"public_key"`
}

type KeyShareResponse struct {
	KeyShare []byte `json:"key_share"`
	Index    uint32 `json:"index"`
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

func (c *Client) GetPublicKey(ctx context.Context) ([]byte, error) {
	resp := &PublicKeyResponse{}

	if err := c.doRequest(ctx, "GET", "/public-key", nil, resp); err != nil {
		return nil, err
	}

	return resp.PublicKey, nil
}

func (c *Client) GetKeyShare(ctx context.Context) ([]byte, uint32, error) {
	resp := &KeyShareResponse{}

	if err := c.doRequest(ctx, "GET", "/key-share", nil, resp); err != nil {
		return nil, 0, err
	}

	return resp.KeyShare, resp.Index, nil
}

func (c *Client) SignRound1(ctx context.Context) ([]byte, []byte, error) {
	resp := &SignRound1Response{}

	if err := c.doRequest(ctx, "POST", "/sign/round1", nil, resp); err != nil {
		return nil, nil, err
	}

	if !resp.Success {
		return nil, nil, fmt.Errorf("sign round1 failed")
	}

	return resp.NonceCommitment, resp.Commitment, nil
}

func (c *Client) SignRound2(ctx context.Context, signingPackage []byte) ([]byte, []byte, error) {
	req := SignRound2Request{SigningPackage: signingPackage}
	resp := &SignRound2Response{}

	if err := c.doRequest(ctx, "POST", "/sign/round2", req, resp); err != nil {
		return nil, nil, err
	}

	if !resp.Success {
		return nil, nil, fmt.Errorf("sign round2 failed")
	}

	return resp.PartialSignature, resp.Commitment, nil
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
