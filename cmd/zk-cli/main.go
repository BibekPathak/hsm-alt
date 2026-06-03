package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourorg/hsm/pkg/zk/policy"
	"github.com/yourorg/hsm/pkg/zk/prover"
	"github.com/yourorg/hsm/pkg/zk/types"
	"github.com/yourorg/hsm/pkg/zk/verifier"
)

var (
	circuitsDir = flag.String("circuits-dir", "./pkg/zk/circuits", "Directory containing circuit artifacts")
	artifactsDir = flag.String("artifacts-dir", "./pkg/zk/artifacts", "Directory for temporary proof artifacts")
)

func main() {
	proveCmd := flag.NewFlagSet("prove", flag.ExitOnError)
	proveAmount := proveCmd.String("amount", "", "Transfer amount (public input)")
	proveLimit := proveCmd.String("limit", "", "Daily limit (private input)")
	proveSpent := proveCmd.String("spent", "", "Spent today (private input)")

	verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
	verifyProofPath := verifyCmd.String("proof", "", "Path to proof JSON file")

	inspectCmd := flag.NewFlagSet("inspect", flag.ExitOnError)
	inspectProofPath := inspectCmd.String("proof", "", "Path to proof JSON file")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "prove":
		args := os.Args[2:]
		circuitName := "transfer_limit"
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			circuitName = args[0]
			args = args[1:]
		}
		proveCmd.Parse(args)
		if *proveAmount == "" || *proveLimit == "" || *proveSpent == "" {
			fmt.Println("Error: --amount, --limit, and --spent are required")
			proveCmd.PrintDefaults()
			os.Exit(1)
		}
		runProve(circuitName, *proveAmount, *proveLimit, *proveSpent)

	case "verify":
		verifyCmd.Parse(os.Args[2:])
		if *verifyProofPath == "" {
			fmt.Println("Error: --proof is required")
			verifyCmd.PrintDefaults()
			os.Exit(1)
		}
		runVerify(*verifyProofPath)

	case "inspect":
		inspectCmd.Parse(os.Args[2:])
		if *inspectProofPath == "" {
			fmt.Println("Error: --proof is required")
			inspectCmd.PrintDefaults()
			os.Exit(1)
		}
		runInspect(*inspectProofPath)

	case "help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printUsage()
	}
}

func printUsage() {
	fmt.Println("zk-cli - ZK Proof Cryptographic Laboratory")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  zk-cli prove <circuit> [flags]")
	fmt.Println("  zk-cli verify [flags]")
	fmt.Println("  zk-cli inspect [flags]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  prove      Generate a ZK proof for a circuit")
	fmt.Println("  verify     Verify a ZK proof")
	fmt.Println("  inspect    Inspect proof contents")
	fmt.Println("  help       Show this help message")
	fmt.Println("")
	fmt.Println("Prove Examples:")
	fmt.Println("  zk-cli prove transfer_limit --amount 500 --limit 10000 --spent 2000")
	fmt.Println("")
	fmt.Println("Verify Examples:")
	fmt.Println("  zk-cli verify --proof proof.json")
	fmt.Println("")
	fmt.Println("Inspect Examples:")
	fmt.Println("  zk-cli inspect --proof proof.json")
}

func runProve(circuitName, amount, limit, spent string) {
	fmt.Printf("Generating proof for circuit: %s\n", circuitName)
	fmt.Printf("  Amount: %s\n", amount)
	fmt.Printf("  Limit: %s\n", limit)
	fmt.Printf("  Spent: %s\n", spent)

	transferLimitPolicy := policy.NewTransferLimitPolicy(limit, spent, *circuitsDir, *artifactsDir)

	proverInstance := prover.New(*circuitsDir, *artifactsDir)
	verifierInstance := verifier.New(*circuitsDir, *artifactsDir)

	policyEngine := policy.New([]types.Policy{transferLimitPolicy}, proverInstance, verifierInstance)

	intent := &types.Intent{
		Value: amount,
	}

	proof, err := policyEngine.EvaluateAndProve(intent)
	if err != nil {
		fmt.Printf("\n❌ Proof generation failed: %v\n", err)
		os.Exit(1)
	}

	output := map[string]interface{}{
		"success":      true,
		"circuit":      circuitName,
		"proof":        json.RawMessage(proof.Data),
		"public_inputs": proof.PublicInputs,
		"generated_at": proof.GeneratedAt.Format(time.RFC3339),
	}

	outputJson, _ := json.MarshalIndent(output, "", "  ")
	fmt.Printf("\n✅ Proof generated successfully!\n\n")
	fmt.Println(string(outputJson))

	proofPath := filepath.Join(*artifactsDir, circuitName+"_proof.json")
	os.WriteFile(proofPath, outputJson, 0644)
	fmt.Printf("\nProof saved to: %s\n", proofPath)
}

func runVerify(proofPath string) {
	fmt.Printf("Verifying proof from: %s\n", proofPath)

	data, err := os.ReadFile(proofPath)
	if err != nil {
		fmt.Printf("\n❌ Failed to read proof file: %v\n", err)
		os.Exit(1)
	}

	var proofData map[string]interface{}
	if err := json.Unmarshal(data, &proofData); err != nil {
		fmt.Printf("\n❌ Failed to parse proof JSON: %v\n", err)
		os.Exit(1)
	}

	circuitName, ok := proofData["circuit"].(string)
	if !ok {
		circuitName = "transfer_limit"
	}

	proofBytes, _ := json.Marshal(proofData["proof"])
	publicInputsRaw, _ := json.Marshal(proofData["public_inputs"])

	var publicInputs []string
	json.Unmarshal(publicInputsRaw, &publicInputs)

	proof := &types.Proof{
		Data:         proofBytes,
		PublicInputs: publicInputs,
		CircuitName:  circuitName,
	}

	verifierInstance := verifier.New(*circuitsDir, *artifactsDir)
	result, err := verifierInstance.Verify(circuitName, proof)
	if err != nil {
		fmt.Printf("\n❌ Verification error: %v\n", err)
		os.Exit(1)
	}

	output := map[string]interface{}{
		"valid":        result.Valid,
		"circuit":      result.CircuitName,
		"verified_at":   result.VerifiedAt.Format(time.RFC3339),
	}

	if !result.Valid {
		output["errors"] = result.Errors
	}

	outputJson, _ := json.MarshalIndent(output, "", "  ")
	if result.Valid {
		fmt.Printf("\n✅ Proof is valid!\n\n")
	} else {
		fmt.Printf("\n❌ Proof is invalid!\n\n")
	}
	fmt.Println(string(outputJson))
}

func runInspect(proofPath string) {
	fmt.Printf("Inspecting proof from: %s\n", proofPath)

	data, err := os.ReadFile(proofPath)
	if err != nil {
		fmt.Printf("\n❌ Failed to read proof file: %v\n", err)
		os.Exit(1)
	}

	var proofData map[string]interface{}
	if err := json.Unmarshal(data, &proofData); err != nil {
		fmt.Printf("\n❌ Failed to parse proof JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n📋 Proof Contents:")
	fmt.Println("──────────────────")

	if circuit, ok := proofData["circuit"].(string); ok {
		fmt.Printf("Circuit:     %s\n", circuit)
	}
	if success, ok := proofData["success"].(bool); ok {
		fmt.Printf("Success:     %v\n", success)
	}
	if generatedAt, ok := proofData["generated_at"].(string); ok {
		fmt.Printf("Generated:   %s\n", generatedAt)
	}
	if publicInputs, ok := proofData["public_inputs"].([]interface{}); ok {
		fmt.Printf("Public Inputs (%d):\n", len(publicInputs))
		for i, input := range publicInputs {
			fmt.Printf("  [%d] %v\n", i, input)
		}
	}
	if proof, ok := proofData["proof"].(map[string]interface{}); ok {
		fmt.Println("Proof Data:")
		for key, value := range proof {
			switch v := value.(type) {
			case []interface{}:
				fmt.Printf("  %s: [%d elements]\n", key, len(v))
			default:
				fmt.Printf("  %s: %v\n", key, v)
			}
		}
	}
	fmt.Println("──────────────────")
}