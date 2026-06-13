#!/bin/bash
# Run enclave on specified port
PORT=${1:-7001}
NODE_ID=${2:-1}
DATA_DIR=/tmp/hsm-node-${NODE_ID}-data mkdir -p $DATA_DIR
cd /home/BibekP/hsm
ENCLAVE_PORT=$PORT NODE_ID=$NODE_ID ./cmd/mpc-enclave/target/release/mpc-enclave
