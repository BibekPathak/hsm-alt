#!/bin/bash
# ZK Circuit Setup Script
# This script sets up the ZK development environment for Circom + SnarkJS

set -e

ZK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CIRCUIT_DIR="$ZK_DIR/circuits"
ARTIFACTS_DIR="$ZK_DIR/circuits/artifacts"
POWERS_OF_TAU_DIR="$ZK_DIR/artifacts"

echo "=========================================="
echo "ZK Circuit Development Environment Setup"
echo "=========================================="
echo ""

# Check prerequisites
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

echo "[1/5] Checking prerequisites..."

if ! command_exists cargo; then
    echo "ERROR: Rust (cargo) is required to build Circom"
    echo "Install from: https://rustup.rs"
    exit 1
fi

if ! command_exists npm; then
    echo "ERROR: npm is required for SnarkJS"
    echo "Install Node.js from: https://nodejs.org"
    exit 1
fi

echo "  ✓ Rust (cargo) found"
echo "  ✓ npm found"

# Install Circom
echo ""
echo "[2/5] Setting up Circom compiler..."

if command_exists circom; then
    CIRCOM_VERSION=$(circom --version 2>&1 || echo "unknown")
    echo "  ✓ Circom already installed: $CIRCOM_VERSION"
else
    echo "  Installing Circom compiler..."

    CIRCOM_TMP=$(mktemp -d)
    cd "$CIRCOM_TMP"

    # Clone circom compiler repository
    if git clone --depth 1 https://github.com/iden3/circom.git circom_temp 2>/dev/null; then
        cd circom_temp
        cargo build --release 2>&1 | tail -5
        cargo install --path . 2>&1 | tail -5

        echo "  ✓ Circom installed successfully"
    else
        echo "  WARNING: Could not install Circom automatically"
        echo "  Please install manually: https://docs.circom.io/getting-started/installation/"
    fi

    cd "$ZK_DIR"
    rm -rf "$CIRCOM_TMP"
fi

# Install SnarkJS
echo ""
echo "[3/5] Setting up SnarkJS..."

if command_exists snarkjs; then
    SNARKJS_VERSION=$(snarkjs --version 2>&1 || echo "unknown")
    echo "  ✓ SnarkJS already installed: $SNARKJS_VERSION"
else
    echo "  Installing SnarkJS globally..."
    npm install -g snarkjs 2>&1 | tail -3

    if command_exists snarkjs; then
        echo "  ✓ SnarkJS installed successfully"
    else
        echo "  WARNING: Could not install SnarkJS automatically"
        echo "  Please run: npm install -g snarkjs"
    fi
fi

# Create artifacts directories
echo ""
echo "[4/5] Creating artifacts directories..."
mkdir -p "$ARTIFACTS_DIR"
mkdir -p "$POWERS_OF_TAU_DIR"
echo "  ✓ Artifacts directories created"

# Initialize powers of tau (if not exists)
echo ""
echo "[5/5] Verifying setup..."

if command_exists circom && command_exists snarkjs; then
    echo ""
    echo "=========================================="
    echo "✅ Setup Complete!"
    echo "=========================================="
    echo ""
    echo "Next steps:"
    echo "  1. Compile a circuit: cd $CIRCUIT_DIR && bash compile.sh transfer_limit"
    echo "  2. Generate proof:   ./zk-cli prove transfer_limit --amount 500 --limit 10000"
    echo "  3. Verify proof:      ./zk-cli verify --proof proof.json"
    echo ""
    echo "To use from Go, import: github.com/yourorg/hsm/pkg/zk"
else
    echo ""
    echo "=========================================="
    echo "⚠️  Setup Incomplete"
    echo "=========================================="
    echo ""
    echo "Please install missing tools manually:"
    if ! command_exists circom; then
        echo "  Circom: Follow instructions at https://docs.circom.io/getting-started/installation/"
    fi
    if ! command_exists snarkjs; then
        echo "  SnarkJS: npm install -g snarkjs"
    fi
fi