package policy

import (
	"fmt"

	"github.com/yourorg/hsm/pkg/zk/prover"
	"github.com/yourorg/hsm/pkg/zk/types"
	"github.com/yourorg/hsm/pkg/zk/verifier"
)

type PolicyEngine struct {
	policies []types.Policy
	prover   *prover.Prover
	verifier *verifier.Verifier
}

func New(policies []types.Policy, prover *prover.Prover, verifier *verifier.Verifier) *PolicyEngine {
	return &PolicyEngine{
		policies: policies,
		prover:   prover,
		verifier: verifier,
	}
}

func (e *PolicyEngine) EvaluateAndProve(intent *types.Intent) (*types.Proof, error) {
	for _, policy := range e.policies {
		satisfied, err := policy.Evaluate(intent)
		if err != nil {
			return nil, fmt.Errorf("policy %s: %w", policy.Name(), err)
		}
		if !satisfied {
			return nil, fmt.Errorf("policy %s not satisfied", policy.Name())
		}
	}

	if len(e.policies) == 0 {
		return nil, fmt.Errorf("no policies registered")
	}

	policy := e.policies[0]

	proof, err := policy.GenerateProof(intent)
	if err != nil {
		return nil, fmt.Errorf("proof generation: %w", err)
	}

	result, err := e.verifier.Verify(policy.Name(), proof)
	if err != nil || !result.Valid {
		return nil, fmt.Errorf("proof verification failed: %v", result.Errors)
	}

	return proof, nil
}

func (e *PolicyEngine) RegisterPolicy(policy types.Policy) {
	e.policies = append(e.policies, policy)
}

func (e *PolicyEngine) ListPolicies() []string {
	names := make([]string, len(e.policies))
	for i, p := range e.policies {
		names[i] = p.Name()
	}
	return names
}
