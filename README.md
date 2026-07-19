# HSM — Hardware-Backed MPC Treasury Infrastructure

**HSM** is an open-source, multi-chain custody platform combining **MPC threshold signatures** (FROST-Ed25519), **ZK proof-based policy enforcement** (Groth16), and **intent-based transaction workflows** for Ethereum and Solana.

Private keys are never reconstructed. A 2-of-3 threshold model distributes key material across three enclaves — no single system can sign alone.

---

## Architecture

```
                    UI / CLI
                        │
                 Wallet Service
                        │
                 Policy Engine  (ZK proof generation + verification)
                        │
                 Signer Resolver
                   /         \
            Direct Signer   MPC Coordinator
                                │
                        ┌───────┼───────┐
                      Node 1  Node 2  Node 3
                        │       │       │
                    Enclave  Enclave  Enclave   (Rust, FROST)
                        │       │       │
                     Ethereum / Solana (blockchain)
```

| Component | Description |
|-----------|-------------|
| **Wallet Service** | REST API server (Chi router, port 8080) — wallet CRUD, intent lifecycle, ZK policy gate, signer resolution |
| **Intent Engine** | Declarative transaction definitions with approval workflows, retry logic, and expiration |
| **Policy Engine** | ZK compliance gate using Groth16 proofs — transfer limits, allowlists, proof verification before execution |
| **Signer Resolver** | Routes to direct Ed25519/ECDSA signer or MPC coordinator based on account type |
| **MPC Nodes** | 3 Go gRPC servers — peer handshake, DKG coordination, sign coordination, heartbeats |
| **Enclaves** | 3 Rust HTTP servers — FROST-Ed25519 DKG and signing, Argon2id key derivation, AES-256-GCM share encryption |
| **Dashboard** | Next.js 14 admin panel — wallet management, intent tracking, fee estimation |

---

## Quick Start

```bash
# Start the full cluster
docker compose -f configs/docker/docker-compose.yml up -d

# Create a wallet
curl -X POST http://localhost:8080/wallet/create \
  -H 'Content-Type: application/json' \
  -d '{"name":"My Treasury"}'

# Create an intent
curl -X POST http://localhost:8080/intent \
  -H 'Content-Type: application/json' \
  -d '{
    "wallet_id": "<wallet_id>",
    "chain": "solana",
    "to": "<recipient>",
    "value": "0.01"
  }'

# Approve and execute
curl -X POST http://localhost:8080/intent/<id>/approve \
  -H 'Content-Type: application/json' \
  -d '{"approver":"admin"}'

curl -X POST http://localhost:8080/intent/<id>/execute
```

---

## Project Structure

```
cmd/
├── wallet-service/     REST API server (Chi router, port 8080)
├── mpc-node/           Go gRPC node (ports 8001-8003)
├── mpc-cli/            CLI for DKG init, signing, key status
├── zk-cli/             CLI for ZK proof generation and verification
├── cli/                General HSM CLI
└── mpc-enclave/        Rust enclave (axum HTTP, ports 7001-7003)

pkg/
├── signer/             Signer interface, ECDSA, Solana (Ed25519), MPC solver
├── wallet/             Wallet/account models, encrypted key store, intent store
├── transaction/        Transaction service with SolanaTxSigner interface
├── mpc/
│   ├── node/           MPC node, sign orchestrator, DKG orchestrator, share store
│   └── protocol/       Protocol types, session state machines
├── blockchain/
│   ├── ethereum/       RPC client, TxBuilder (EIP-1559, ERC-20)
│   └── solana/         RPC client, TxBuilder (native, SPL tokens)
├── zk/
│   ├── types/          Policy interface, Proof, Intent models
│   ├── prover/         SnarkJS proof generation wrapper
│   ├── verifier/       SnarkJS verification wrapper
│   └── policy/         Policy engine with transfer limit policy
└── config/             NodeConfig loader from YAML

ui/
├── dashboard/          Next.js 14 admin panel (React Query, Tailwind)
└── landing/            Next.js 15 marketing site

api/                    Protobuf definitions (3 services, 21 RPCs)
configs/                YAML configs, Dockerfiles, K8s manifests
```

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| API Server | Go, Chi router |
| gRPC | protobuf, 3 services, 21 RPCs |
| MPC Cryptography | Rust, FROST-Ed25519, AES-256-GCM, Argon2id |
| ZK Proofs | Circom, SnarkJS, Groth16 on BN128 |
| Ethereum | go-ethereum, EIP-1559, ERC-20 |
| Solana | native RPC, SPL tokens, ATA management |
| Frontend | Next.js 14/15, React Query, Tailwind CSS |
| Deployment | Docker, docker-compose, Kubernetes |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Wallet service HTTP port |
| `ETHEREUM_RPC` | Sepolia public | Ethereum RPC endpoint |
| `SOLANA_RPC` | Devnet | Solana RPC endpoint |
| `ENCRYPTION_PASSWORD` | `hsm-default-password` | Key encryption password |
| `MPC_ENABLED` | — | Enable MPC signing mode |
| `MPC_PEERS` | — | Comma-separated peer addresses |
| `MPC_THRESHOLD` | `2` | Signing threshold (2-of-3) |
| `MPC_SHARE_PASSWORD` | — | Share encryption password |
| `ZK_CIRCUITS_DIR` | `./pkg/zk/circuits` | Circom circuit directory |
| `ZK_ARTIFACTS_DIR` | `./pkg/zk/artifacts` | Proving/verification keys |

---

## Flows

### Wallet Creation (MPC)

```
POST /wallet/create { "signer_type": "mpc_solana" }
  → Account stored with SignerType="mpc_solana"
  → Public key from DKG cluster
  → No private key stored
```

### Intent Execution (MPC)

```
POST /intent/{id}/execute
  → Resolve account type
  → ZK policy gate (proof generation + verification)
  → MPC coordinator signs via 2-round protocol
  → Aggregate partial signatures
  → Broadcast to blockchain
```

### Distributed Key Generation

```
mpc-cli dkg init --cluster-id "wallet_123" --peers localhost:8001,...
  → Round 1: Each node generates secret package, shares commitment
  → Round 2: Each node processes round1 packages, generates verification shares
  → Round 3: Complete DKG, derive group public key + local key share
  → Save encrypted share to disk (Argon2id + AES-256-GCM)
```

---

## Security

| Threat | Mitigation |
|--------|-----------|
| Single node failure | 2-of-3 threshold enables signing with 2 nodes |
| Node compromise | Single key share cannot reconstruct private key |
| Malicious partial signature | FROST validates commitments before aggregation |
| Replay attack | Intent IDs, nonces, chain-specific hashes |
| Policy bypass | Groth16 proof verification gates execution |
| Double execution | Intent state machine + execution locks |
| Database compromise | Private keys never stored centrally |

---

## Roadmap

### Past
- ✅ Wallet Service — multi-chain wallet CRUD, encrypted key storage
- ✅ Ethereum — native ETH, ERC-20, EIP-1559
- ✅ Solana — native SOL, SPL tokens
- ✅ Intent Engine — declarative intents, approval workflows
- ✅ ZK Policy Engine — Groth16 compliance proofs
- ✅ MPC Infrastructure — 2-of-3 FROST-Ed25519, DKG, encrypted shares

### Next
- TEE Integration — hardware-backed attestation
- Threshold ECDSA — Ethereum MPC via FROST-secp256k1
- Hardware HSM — support for hardware security modules
- Policy Framework — extensible, configurable policies
- Audit API — structured audit log export
- FROST-secp256k1 — Ethereum threshold signing

---

## Development

```bash
# Build everything
make build

# Run tests
make test

# Start local cluster
docker compose -f configs/docker/docker-compose.yml up -d

# Run DKG
MPC_SHARE_PASSWORD=secret go run ./cmd/mpc-cli/ dkg init \
  --cluster-id dev \
  --peers localhost:8001,localhost:8002,localhost:8003

# Start wallet service
go run ./cmd/wallet-service/

# Start landing page
cd ui/landing && npm run dev

# Start dashboard
cd ui/dashboard && npx next dev -p 3001
```

---

## License

MIT
