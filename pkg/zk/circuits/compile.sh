#!/bin/bash
# Compile Circom circuit and generate proving/verification keys

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CIRCUIT_NAME="${1:-transfer_limit}"
CIRCUIT_FILE="$SCRIPT_DIR/${CIRCUIT_NAME}.circom"
ARTIFACTS_DIR="$SCRIPT_DIR/artifacts"
POWERS_OF_TAU_DIR="$SCRIPT_DIR/../artifacts"

CIRCUIT_DIR="$ARTIFACTS_DIR/${CIRCUIT_NAME}"
PTAU_FILE="$POWERS_OF_TAU_DIR/pot12_final.ptau"
PTAU_RAW="$POWERS_OF_TAU_DIR/pot12_0000.ptau"
PTAU_CONTRIB="$POWERS_OF_TAU_DIR/pot12_0001.ptau"
PTAU_PHASE2="$POWERS_OF_TAU_DIR/pot12_phase2.ptau"
PTAU_POWER=12

echo "============================================"
echo "ZK Circuit Compilation: $CIRCUIT_NAME"
echo "============================================"
echo ""

if [ ! -f "$CIRCUIT_FILE" ]; then
    echo "ERROR: Circuit file not found: $CIRCUIT_FILE"
    exit 1
fi

mkdir -p "$CIRCUIT_DIR"
mkdir -p "$ARTIFACTS_DIR"
mkdir -p "$POWERS_OF_TAU_DIR"

echo "[1/6] Checking prerequisites..."

if ! command -v circom >/dev/null 2>&1; then
    echo "ERROR: circom not found. Run setup.sh first or install manually."
    exit 1
fi

if ! command -v snarkjs >/dev/null 2>&1; then
    echo "ERROR: snarkjs not found. Run setup.sh first or install manually."
    exit 1
fi

echo "  ✓ circom found"
echo "  ✓ snarkjs found"

echo ""
echo "[2/6] Initializing Powers of Tau..."

if [ ! -f "$PTAU_FILE" ]; then
    echo "  Creating new Powers of Tau (2^$PTAU_POWER constraints)..."
    snarkjs powersoftau new bn128 $PTAU_POWER "$PTAU_RAW" -v 2>&1 | tail -3
    echo "  ✓ Powers of Tau initialized"

    echo "  Contributing to Powers of Tau..."
    snarkjs powersoftau contribute "$PTAU_RAW" "$PTAU_CONTRIB" -n="First contribution" -v -e="random entropy" 2>&1 | tail -3
    echo "  ✓ Contribution done"

    echo "  Preparing phase 2..."
    snarkjs powersoftau prepare phase2 "$PTAU_CONTRIB" "$PTAU_FILE" -v 2>&1 | tail -3
    echo "  ✓ Phase 2 ready"
else
    echo "  ✓ Using existing Powers of Tau: $PTAU_FILE"
fi

echo ""
echo "[3/6] Compiling circuit..."

cd "$SCRIPT_DIR"

CIRCUIT_OUTPUT="$CIRCUIT_DIR/${CIRCUIT_NAME}"
circom "$CIRCUIT_FILE" \
    --wasm \
    --r1cs \
    --output "$CIRCUIT_DIR" 2>&1 | tail -10

if [ ! -f "$CIRCUIT_DIR/${CIRCUIT_NAME}.r1cs" ]; then
    echo "ERROR: Circuit compilation failed"
    exit 1
fi

echo "  ✓ Circuit compiled: ${CIRCUIT_NAME}.r1cs"
echo "  ✓ WASM output: ${CIRCUIT_NAME}_js/${CIRCUIT_NAME}.wasm"

echo ""
echo "[4/6] Generating proving key (zkey)..."

snarkjs groth16 setup \
    "$CIRCUIT_DIR/${CIRCUIT_NAME}.r1cs" \
    "$PTAU_FILE" \
    "$CIRCUIT_DIR/${CIRCUIT_NAME}_final.zkey" 2>&1 | tail -3

echo "  ✓ Proving key (zkey) generated"

echo ""
echo "[5/6] Generating verification key..."

snarkjs zkey export verificationkey \
    "$CIRCUIT_DIR/${CIRCUIT_NAME}_final.zkey" \
    "$CIRCUIT_DIR/${CIRCUIT_NAME}_vkey.json" 2>&1 | tail -3

echo "  ✓ Verification key generated"

echo ""
echo "[6/6] Writing circuit info..."

cat > "$CIRCUIT_DIR/circuit.json" <<EOF
{
  "name": "$CIRCUIT_NAME",
  "version": 1,
  "r1cs": "${CIRCUIT_NAME}.r1cs",
  "wasm": "${CIRCUIT_NAME}_js/${CIRCUIT_NAME}.wasm",
  "proving_key": "${CIRCUIT_NAME}_pkey.json",
  "verification_key": "${CIRCUIT_NAME}_vkey.json",
  "compiled_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

echo "  ✓ Circuit info saved"

echo ""
echo "============================================"
echo "✅ Compilation Complete!"
echo "============================================"
echo ""
echo "Artifacts saved to: $CIRCUIT_DIR/"
echo ""
ls -la "$CIRCUIT_DIR/"
echo ""
echo "Next steps:"
echo "  Generate proof:   zk-cli prove $CIRCUIT_NAME --amount 500 --limit 10000 --spent 2000"
echo "  Verify proof:     zk-cli verify --proof proof.json"