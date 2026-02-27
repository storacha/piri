VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X github.com/storacha/piri/pkg/build.version=$(VERSION) -X github.com/storacha/piri/pkg/build.Commit=$(COMMIT) -X github.com/storacha/piri/pkg/build.Date=$(DATE) -X github.com/storacha/piri/pkg/build.BuiltBy=make"
TAGS?=
DOCKER?=$(shell which docker)

.PHONY: all build install test clean calibnet mockgen check-docs-links piri-prod piri-debug docker-setup docker-prod docker-dev

all: build

build: piri

# piri depends on Go sources - use shell to check if rebuild needed
piri: FORCE
	@if [ ! -f piri ] || \
	   [ -n "$$(find cmd pkg internal -name '*.go' -type f -newer piri 2>/dev/null)" ]; then \
		echo "Building piri..."; \
		go build $(GOFLAGS) $(TAGS) -o ./piri github.com/storacha/piri/cmd; \
	fi

FORCE:

install:
	go install ./cmd/storage

test:
	go test ./...

clean:
	rm -f ./piri

mockgen:
	mockgen -source=./pkg/pdp/aggregator/interface.go -destination=./internal/mocks/aggregator.go -package=mocks
	mockgen -source=./pkg/pdp/types/api.go -destination=./internal/mocks/pdp_api.go -package=mocks
	mockgen -source=./internal/ipldstore/ipldstore.go -destination=./internal/mocks/ipldstore.go -package=mocks
	mockgen -source=./pkg/pdp/aggregator/steps.go -destination=./internal/mocks/steps.go -package=mocks
	mockgen -destination=./internal/mocks/sender_eth_client.go -package=mocks github.com/storacha/piri/pkg/pdp/tasks SenderETHClient
	mockgen -destination=./internal/mocks/message_watcher_eth_client.go -package=mocks github.com/storacha/piri/pkg/pdp/tasks MessageWatcherEthClient
	mockgen -destination=./internal/mocks/contract_backend.go -package=mocks github.com/ethereum/go-ethereum/accounts/abi/bind ContractBackend
	mockgen -source=./pkg/pdp/smartcontracts/contract.go -destination=./pkg/pdp/smartcontracts/mocks/pdp.go -package=mocks

# Contract generation targets
.PHONY: generate-contracts clean-contracts

generate-contracts:
	cd pkg/pdp/smartcontracts && ./generate.sh

clean-contracts:
	rm -rf pkg/pdp/smartcontracts/abis
	rm -rf pkg/pdp/smartcontracts/bindings
	rm -f pkg/pdp/smartcontracts/mocks/*.go

mockgen-contracts: generate-contracts
	mockgen -source=./pkg/pdp/smartcontracts/contract.go -destination=./pkg/pdp/smartcontracts/mocks/pdp.go -package=mocks


# special target that sets the calibnet tag and invokes build
calibnet: TAGS=-tags calibnet
calibnet: build

# Production binary - stripped symbols for smaller size
piri-prod: FORCE
	@echo "Building piri (production)..."
	go build -ldflags="-s -w -X github.com/storacha/piri/pkg/build.version=$(VERSION) -X github.com/storacha/piri/pkg/build.Commit=$(COMMIT) -X github.com/storacha/piri/pkg/build.Date=$(DATE) -X github.com/storacha/piri/pkg/build.BuiltBy=make" $(TAGS) -o ./piri github.com/storacha/piri/cmd

# Debug binary - no optimizations, no inlining, full symbols
piri-debug: FORCE
	@echo "Building piri (debug)..."
	go build -gcflags="all=-N -l" -ldflags="-X github.com/storacha/piri/pkg/build.version=$(VERSION) -X github.com/storacha/piri/pkg/build.Commit=$(COMMIT) -X github.com/storacha/piri/pkg/build.Date=$(DATE) -X github.com/storacha/piri/pkg/build.BuiltBy=make-debug" $(TAGS) -o ./piri github.com/storacha/piri/cmd

# Docker targets (multi-arch: amd64 + arm64)
docker-setup:
	$(DOCKER) buildx create --name multiarch --use 2>/dev/null || $(DOCKER) buildx use multiarch

docker-prod: docker-setup
	$(DOCKER) buildx build --platform linux/amd64,linux/arm64 --target prod -t piri:latest .

docker-dev: docker-setup
	$(DOCKER) buildx build --platform linux/amd64,linux/arm64 --target dev -t piri:dev .

# Check for broken links in documentation
check-docs-links:
	@./scripts/check-docs-links.sh
