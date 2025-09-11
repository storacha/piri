VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X github.com/storacha/piri/pkg/build.version=$(VERSION) -X github.com/storacha/piri/pkg/build.Commit=$(COMMIT) -X github.com/storacha/piri/pkg/build.Date=$(DATE) -X github.com/storacha/piri/pkg/build.BuiltBy=make"
TAGS?=

.PHONY: all build piri install test clean calibnet mockgen tools check-docs-links generate-contracts verify-contracts contracts-update

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
	mockgen -destination=./internal/mocks/pdp.go -package=mocks github.com/storacha/piri/pkg/pdp/service/contract PDP
	mockgen -destination=./internal/mocks/pdp_proving_schedule.go -package=mocks github.com/storacha/piri/pkg/pdp/service/contract PDPProvingSchedule
	mockgen -destination=./internal/mocks/pdp_verifier.go -package=mocks github.com/storacha/piri/pkg/pdp/service/contract PDPVerifier

# Install development tools
tools:
	@echo "Installing development tools..."
	go install github.com/ethereum/go-ethereum/cmd/abigen@latest
	@echo "Tools installed to $$(go env GOPATH)/bin/"
	@if command -v abigen >/dev/null 2>&1; then \
		echo "✓ abigen is available in PATH"; \
	else \
		echo "⚠ abigen installed but not in PATH. Add $$(go env GOPATH)/bin to your PATH:"; \
		echo "  export PATH=\$$PATH:$$(go env GOPATH)/bin"; \
	fi

# Contract generation targets
generate-contracts:
	@echo "Generating contract bindings..."
	@./scripts/generate-contracts.sh

verify-contracts: generate-contracts
	@echo "Verifying contract bindings are up-to-date..."
	@if ! git diff --exit-code pkg/pdp/service/contract/internal/ pkg/pdp/service/contract/VERSION; then \
		echo "❌ Generated contract files are out of date. Run 'make generate-contracts' and commit the changes."; \
		echo "Changed files:"; \
		git diff --name-only pkg/pdp/service/contract/internal/ pkg/pdp/service/contract/VERSION; \
		exit 1; \
	else \
		echo "✅ Contract bindings are up-to-date."; \
	fi

contracts-update:
	@echo "Updating contract submodule to latest version..."
	@git submodule update --remote contracts/pdp
	@echo "Regenerating contract bindings..."
	@$(MAKE) generate-contracts
	@echo "Contract submodule updated and bindings regenerated."
	@echo "Review changes and commit if satisfied:"
	@echo "  git add contracts/pdp pkg/pdp/service/contract/"
	@echo "  git commit -m 'Update PDP contracts to latest version'"

# special target that sets the calibnet tag and invokes build
calibnet: TAGS=-tags calibnet
calibnet: build

# Check for broken links in documentation
check-docs-links:
	@./scripts/check-docs-links.sh
