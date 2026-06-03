package prover

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/yourorg/hsm/pkg/zk/types"
)

type Prover struct {
	circuitsDir  string
	artifactsDir string
}

func New(circuitsDir, artifactsDir string) *Prover {
	return &Prover{
		circuitsDir:  circuitsDir,
		artifactsDir: artifactsDir,
	}
}

type ProofInput struct {
	PublicInputs  map[string]string
	PrivateInputs map[string]string
}

func (p *Prover) GenerateProof(circuitName string, input ProofInput) (*types.Proof, error) {
	circuitDir := filepath.Join(p.circuitsDir, "artifacts", circuitName)
	wasmPath := filepath.Join(circuitDir, circuitName+"_js", circuitName+".wasm")
	pkeyPath := filepath.Join(circuitDir, circuitName+"_final.zkey")

	for _, path := range []string{wasmPath, pkeyPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("artifacts not found: %s", path)
		}
	}

	inputJson, err := json.Marshal(map[string]interface{}{
		"transfer_amount": input.PublicInputs["transfer_amount"],
		"daily_limit":     input.PrivateInputs["daily_limit"],
		"spent_today":     input.PrivateInputs["spent_today"],
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inputs: %w", err)
	}

	inputPath := filepath.Join(p.artifactsDir, circuitName+"_input.json")
	if err := os.WriteFile(inputPath, inputJson, 0600); err != nil {
		return nil, fmt.Errorf("failed to write input file: %w", err)
	}
	defer os.Remove(inputPath)

	proofPath := filepath.Join(p.artifactsDir, circuitName+"_proof.json")
	pubInputPath := filepath.Join(p.artifactsDir, circuitName+"_pub.json")

	pubInputJson, _ := json.Marshal([]string{input.PublicInputs["transfer_amount"]})
	if err := os.WriteFile(pubInputPath, pubInputJson, 0600); err != nil {
		return nil, fmt.Errorf("failed to write public input file: %w", err)
	}
	defer os.Remove(pubInputPath)

	cmd := exec.Command("snarkjs", "groth16", "fullprove",
		inputPath,
		wasmPath,
		pkeyPath,
		proofPath,
		pubInputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("snarkjs proof generation failed: %w\noutput: %s", err, string(output))
	}

	proofData, err := os.ReadFile(proofPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof file: %w", err)
	}
	defer os.Remove(proofPath)

	pubData, err := os.ReadFile(pubInputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public input file: %w", err)
	}
	defer os.Remove(pubInputPath)

	var publicInputs []string
	if err := json.Unmarshal(pubData, &publicInputs); err != nil {
		return nil, fmt.Errorf("failed to parse public inputs: %w", err)
	}

	return &types.Proof{
		Data:         proofData,
		PublicInputs: publicInputs,
		CircuitName:  circuitName,
		GeneratedAt:  time.Now().UTC(),
	}, nil
}

func (p *Prover) Verify(circuitName string, proof *types.Proof) (*types.VerificationResult, error) {
	circuitDir := filepath.Join(p.circuitsDir, "artifacts", circuitName)
	vkeyPath := filepath.Join(circuitDir, circuitName+"_vkey.json")

	if _, err := os.Stat(vkeyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("verification key not found: %s", vkeyPath)
	}

	pubInputJson, _ := json.Marshal(proof.PublicInputs)
	pubInputPath := filepath.Join(p.artifactsDir, circuitName+"_pub_verify.json")
	if err := os.WriteFile(pubInputPath, pubInputJson, 0600); err != nil {
		return nil, fmt.Errorf("failed to write public input: %w", err)
	}
	defer os.Remove(pubInputPath)

	proofJson := map[string]interface{}{}
	if err := json.Unmarshal(proof.Data, &proofJson); err != nil {
		return nil, fmt.Errorf("invalid proof data: %w", err)
	}
	proofPath := filepath.Join(p.artifactsDir, circuitName+"_proof_verify.json")
	proofBytes, _ := json.Marshal(proofJson)
	if err := os.WriteFile(proofPath, proofBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to write proof: %w", err)
	}
	defer os.Remove(proofPath)

	callArgs := []string{"groth16", "verify", vkeyPath, pubInputPath, proofPath}
	cmd := exec.Command("snarkjs", callArgs...)

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
