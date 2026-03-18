package enclave

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	addr string
	conn *grpc.ClientConn
}

func NewClient(addr string) (*Client, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to enclave: %w", err)
	}

	return &Client{
		addr: addr,
		conn: conn,
	}, nil
}

func (c *Client) Initialize(ctx context.Context, clusterID string, threshold, totalShares uint32) error {
	return nil
}

func (c *Client) GetStatus(ctx context.Context) (string, uint32, []byte, bool, error) {
	return "ready", 1, []byte{}, true, nil
}

func (c *Client) StartDKG(ctx context.Context, participants []uint32) error {
	return nil
}

func (c *Client) GetPublicKey(ctx context.Context) ([]byte, error) {
	return []byte{}, nil
}

func (c *Client) Sign(ctx context.Context, message []byte, signers []uint32) ([]byte, error) {
	return []byte{}, nil
}

func (c *Client) EvolveKey(ctx context.Context) (uint32, []byte, []byte, error) {
	return 1, []byte{}, []byte{}, nil
}

func (c *Client) GetAttestation(ctx context.Context) ([]byte, []byte, error) {
	return []byte{}, []byte{}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
