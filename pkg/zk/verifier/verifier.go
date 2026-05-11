package verifier

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/yourorg/hsm/pkg/zk/types"
)

type Verifier struct {
	circuitsDir  string
	artifactsDir string
}

func New(circuitsDir, artifactsDir string) *Verifier {
	return &Verifier{
		circuitsDir:  circuitsDir,
		artifactsDir: artifactsDir,
	}
}

func (v *Verifier) Verify(circuitName string, proof *types.Proof) (*types.VerificationResult, error) {
	circuitDir := filepath.Join(v.circuitsDir, circuitName)
	vkeyPath := filepath.Join(circuitDir, circuitName+"_vkey.json")

	if _, err := os.Stat(vkeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("verification key not found: %s (run compile.sh first)", vkeyPath)
	}

	pubInputJson, _ := json.Marshal(proof.PublicInputs)
	pubInputPath := filepath.Join(v.artifactsDir, circuitName+"_pub_verify.json")
	if err := os.WriteFile(pubInputPath, pubInputJson, 0600); err != nil {
		return nil, fmt.Errorf("failed to write public input: %w", err)
	}
	defer os.Remove(pubInputPath)

	proofJson := map[string]interface{}{}
	if err := json.Unmarshal(proof.Data, &proofJson); err != nil {
		return nil, fmt.Errorf("invalid proof data: %w", err)
	}
	proofPath := filepath.Join(v.artifactsDir, circuitName+"_proof_verify.json")
	proofBytes, _ := json.Marshal(proofJson)
	if err := os.WriteFile(proofPath, proofBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to write proof: %w", err)
	}
	defer os.Remove(proofPath)

	cmd := exec.Command("snarkjs", "groth16", "verify", vkeyPath, pubInputPath, proofPath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &types.VerificationResult{
			Valid:       false,
			CircuitName: circuitName,
			VerifiedAt:  time.Now().UTC(),
			Errors:      []string{string(output)},
		}, nil
	}

	return &types.VerificationResult{
		Valid:       true,
		CircuitName: circuitName,
		VerifiedAt:  time.Now().UTC(),
	}, nil
}

func (v *Verifier) LoadVerificationKey(circuitName string) (map[string]interface{}, error) {
	circuitDir := filepath.Join(v.circuitsDir, circuitName)
	vkeyPath := filepath.Join(circuitDir, circuitName+"_vkey.json")

	data, err := os.ReadFile(vkeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read verification key: %w", err)
	}

	var vkey map[string]interface{}
	if err := json.Unmarshal(data, &vkey); err != nil {
		return nil, fmt.Errorf("failed to parse verification key: %w", err)
	}

	return vkey, nil
}