package policy

import (
	"fmt"
	"strconv"

	"github.com/yourorg/hsm/pkg/zk/prover"
	"github.com/yourorg/hsm/pkg/zk/types"
)

type TransferLimitPolicy struct {
	Limit        uint64
	SpentToday   uint64
	CircuitsDir  string
	ArtifactsDir string
}

func NewTransferLimitPolicy(limit, spentToday, circuitsDir, artifactsDir string) *TransferLimitPolicy {
	limitVal, _ := strconv.ParseUint(limit, 10, 64)
	spentVal, _ := strconv.ParseUint(spentToday, 10, 64)

	return &TransferLimitPolicy{
		Limit:        limitVal,
		SpentToday:   spentVal,
		CircuitsDir:  circuitsDir,
		ArtifactsDir: artifactsDir,
	}
}

func (p *TransferLimitPolicy) Name() string {
	return "transfer_limit"
}

func (p *TransferLimitPolicy) Evaluate(intent *types.Intent) (bool, error) {
	amount, err := strconv.ParseUint(intent.Value, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid intent value: %w", err)
	}

	if p.SpentToday >= p.Limit {
		return false, fmt.Errorf("daily limit already exhausted: spent %d, limit %d", p.SpentToday, p.Limit)
	}

	remaining := p.Limit - p.SpentToday
	return amount <= remaining, nil
}

func (p *TransferLimitPolicy) GenerateProof(intent *types.Intent) (*types.Proof, error) {
	amount, err := strconv.ParseUint(intent.Value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid intent value: %w", err)
	}

	if amount > p.Limit-p.SpentToday {
		return nil, fmt.Errorf("transfer amount exceeds remaining daily limit")
	}

	proverInstance := prover.New(p.CircuitsDir, p.ArtifactsDir)

	input := prover.ProofInput{
		PublicInputs: map[string]string{
			"transfer_amount": intent.Value,
		},
		PrivateInputs: map[string]string{
			"daily_limit": strconv.FormatUint(p.Limit, 10),
			"spent_today": strconv.FormatUint(p.SpentToday, 10),
		},
	}

	return proverInstance.GenerateProof(p.Name(), input)
}

func (p *TransferLimitPolicy) PublicInputs(intent *types.Intent) ([]string, error) {
	return []string{intent.Value}, nil
}

func (p *TransferLimitPolicy) PrivateInputs(intent *types.Intent) (map[string]string, error) {
	return map[string]string{
		"daily_limit": strconv.FormatUint(p.Limit, 10),
		"spent_today": strconv.FormatUint(p.SpentToday, 10),
	}, nil
}

func (p *TransferLimitPolicy) UpdateSpent(amount uint64) {
	p.SpentToday += amount
}
