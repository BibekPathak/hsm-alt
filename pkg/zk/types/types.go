package types

import (
	"fmt"
	"time"
)

type Policy interface {
	Name() string
	Evaluate(intent *Intent) (bool, error)
	GenerateProof(intent *Intent) (*Proof, error)
	PublicInputs(intent *Intent) ([]string, error)
	PrivateInputs(intent *Intent) (map[string]string, error)
}

type Intent struct {
	WalletID string
	Chain    string
	To       string
	Value    string
	GasLimit uint64
}

type Proof struct {
	Data         []byte    `json:"proof"`
	PublicInputs []string  `json:"public_inputs"`
	CircuitName  string    `json:"circuit_name"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type PublicInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PrivateInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CircuitConfig struct {
	Name           string
	Version        int
	InputArtifacts CircuitArtifacts
}

type CircuitArtifacts struct {
	ProvingKeyPath   string
	VerifyingKeyPath string
	WasmPath         string
}

type VerificationResult struct {
	Valid       bool      `json:"valid"`
	CircuitName string    `json:"circuit_name"`
	VerifiedAt  time.Time `json:"verified_at"`
	Errors      []string  `json:"errors,omitempty"`
}

func (v *VerificationResult) Error() error {
	if v.Valid {
		return nil
	}
	return fmt.Errorf("verification failed: %v", v.Errors)
}

type ProofRequest struct {
	CircuitName   string
	PublicInputs  map[string]string
	PrivateInputs map[string]string
}

type ProofResponse struct {
	Proof *Proof
	Error error
}

const (
	CurveBN128    = "bn128"
	CurveBLS12381 = "bls12-381"
)

type Curve string

const (
	DefaultCurve = CurveBN128
)

type ZKLibrary string

const (
	SnarkJS ZKLibrary = "snarkjs"
	Halo2   ZKLibrary = "halo2"
	Gnark   ZKLibrary = "gnark"
)
