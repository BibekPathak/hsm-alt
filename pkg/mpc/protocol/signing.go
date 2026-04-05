package protocol

import "context"

type SigningMessage struct {
	Type      string
	FromNode  uint32
	ToNode    uint32
	Message   []byte
	Signature []byte
}

type SigningCoordinator struct {
	threshold  uint32
	totalNodes uint32
}

func NewSigningCoordinator(threshold, totalNodes uint32) *SigningCoordinator {
	return &SigningCoordinator{
		threshold:  threshold,
		totalNodes: totalNodes,
	}
}

func (c *SigningCoordinator) CollectSignatures(ctx context.Context, message []byte, signers []uint32, clients map[uint32]NodeClient) ([]byte, error) {
	type result struct {
		nodeID   uint32
		sigBytes []byte
		err      error
	}

	results := make(chan result, len(signers))

	for _, nodeID := range signers {
		client, ok := clients[nodeID]
		if !ok {
			continue
		}

		go func(id uint32, cli NodeClient) {
			sig, err := cli.SendSignMessage(ctx, &SignMessage{
				FromNode: id,
				Message:  message,
			})
			if err != nil {
				results <- result{nodeID: id, err: err}
				return
			}
			results <- result{nodeID: id, sigBytes: sig.PartialSignature}
		}(nodeID, client)
	}

	var signatures [][]byte
	for i := 0; i < len(signers); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-results:
			if r.err != nil {
				continue
			}
			signatures = append(signatures, r.sigBytes)
		}
	}

	if len(signatures) < int(c.threshold) {
		return nil, ErrNotEnoughSignatures
	}

	return nil, nil
}

var ErrNotEnoughSignatures = &SigningError{"not enough signatures collected"}

type SigningError struct {
	msg string
}

func (e *SigningError) Error() string {
	return e.msg
}
