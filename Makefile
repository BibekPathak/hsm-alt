.PHONY: all build build-enclave build-node test proto docker-build docker-run clean

# Go variables
GO := go
GOCMD := $(GO)
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod

# Rust variables
CARGO := cargo
RUST_TARGET := target/release

# Project variables
BIN_DIR := bin
ENCLAVE_NAME := mpc-enclave
NODE_NAME := mpc_node

all: build

#######################
# Rust Enclave Build #
#######################

build-enclave:
	cd cmd/mpc-enclave && $(CARGO) build --release
	mkdir -p $(BIN_DIR)
	cp cmd/mpc-enclave/target/release/$(ENCLAVE_NAME) $(BIN_DIR)/

build-enclave-sgx: build-enclave
	gramine-clang $(BIN_DIR)/$(ENCLAVE_NAME) -o $(BIN_DIR)/$(ENCLAVE_NAME).sgx \
		-DIsSimulation=true \
		-DIsEnclave=true

###################
# Go Node Build #
###################

build-node:
	cd cmd/mpc-node && $(GOBUILD) -o ../../$(BIN_DIR)/$(NODE_NAME) .
	cd cmd/operator && $(GOBUILD) -o ../../$(BIN_DIR)/operator .
	cd cmd/cli && $(GOBUILD) -o ../../$(BIN_DIR)/hsm-cli .

build: build-enclave build-node

###############
# Proto Build #
###############

proto:
	cd api && $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	cd api && $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	protoc --go_out=. --go_opt=paths=source_relative \
		   --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		   mpc.proto

###########
# Tests #
###########

test:
	cd cmd/mpc-enclave && $(CARGO) test
	$(GOTEST) ./...

test-integration:
	$(GOTEST) -tags=integration ./test/...

###################
# Docker Build #
###################

docker-build:
	docker build -f configs/docker/Dockerfile.enclave -t mpc-enclave:latest .
	docker build -f configs/docker/Dockerfile.node -t mpc-node:latest .

docker-build-all: docker-build
	docker-compose -f configs/docker/docker-compose.yml build

docker-run:
	docker-compose -f configs/docker/docker-compose.yml up -d

docker-stop:
	docker-compose -f configs/docker/docker-compose.yml down

###############
# K8s Deploy #
###############

k8s-deploy:
	kubectl apply -f configs/k8s/manifests/

k8s-delete:
	kubectl delete -f configs/k8s/manifests/

k8s-logs:
	kubectl logs -l app=mpc-node -f

k8s-status:
	kubectl get pods -l app=mpc-node

###############
# Utilities #
###############

clean:
	rm -rf $(BIN_DIR)
	cd cmd/mpc-enclave && $(CARGO) clean
	cd cmd/mpc-node && $(GOCMD) clean
	cd cmd/operator && $(GOCMD) clean
	cd cmd/cli && $(GOCMD) clean

fmt:
	cd cmd/mpc-enclave && $(CARGO) fmt
	$(GOCMD) fmt ./...

lint:
	cd cmd/mpc-enclave && $(CARGO) clippy -- -D warnings
	$(GOLINT) ./...

setup:
	$(GOMOD) download
	cd cmd/mpc-enclave && $(CARGO) fetch

help:
	@echo "Available targets:"
	@echo "  build              - Build both enclave and node"
	@echo "  build-enclave      - Build Rust enclave"
	@echo "  build-node         - Build Go node and operator"
	@echo "  test               - Run all tests"
	@echo "  docker-build       - Build Docker images"
	@echo "  docker-run         - Start Docker cluster"
	@echo "  k8s-deploy         - Deploy to Kubernetes"
	@echo "  clean              - Clean build artifacts"