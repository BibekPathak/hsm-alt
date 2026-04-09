module github.com/yourorg/hsm

go 1.25.0

require (
	github.com/ethereum/go-ethereum v1.17.1
	github.com/go-chi/chi/v5 v5.2.5
	github.com/go-chi/cors v1.2.2
	github.com/google/uuid v1.6.0
	github.com/mr-tron/base58 v1.2.0
	github.com/yourorg/hsm/api v0.0.0
	go.uber.org/zap v1.26.0
	golang.org/x/crypto v0.49.0
	google.golang.org/grpc v1.77.0
)

require (
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/bits-and-blooms/bitset v1.20.0 // indirect
	github.com/consensys/gnark-crypto v0.18.1 // indirect
	github.com/crate-crypto/go-eth-kzg v1.4.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/ethereum/c-kzg-4844/v2 v2.1.6 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/supranational/blst v0.3.16 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/yourorg/hsm/api => ./api
