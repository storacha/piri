VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X github.com/storacha/piri/pkg/build.version=$(VERSION) -X github.com/storacha/piri/pkg/build.Commit=$(COMMIT) -X github.com/storacha/piri/pkg/build.Date=$(DATE) -X github.com/storacha/piri/pkg/build.BuiltBy=make"
TAGS?=

.PHONY: all build piri install test clean calibnet mockgen check-docs-links

all: build

build: piri

piri:
	go build $(GOFLAGS) $(TAGS) -o ./piri github.com/storacha/piri/cmd

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
	rm -f pkg/pdp/smartcontracts/bindings/*.go
	rm -f pkg/pdp/smartcontracts/mocks/*.go

mockgen-contracts: generate-contracts
	mockgen -source=./pkg/pdp/smartcontracts/contract.go -destination=./pkg/pdp/smartcontracts/mocks/pdp.go -package=mocks


# special target that sets the calibnet tag and invokes build
calibnet: TAGS=-tags calibnet
calibnet: build

# Check for broken links in documentation
check-docs-links:
	@./scripts/check-docs-links.sh
