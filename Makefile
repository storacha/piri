VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X github.com/storacha/piri/pkg/build.version=$(VERSION) -X github.com/storacha/piri/pkg/build.Commit=$(COMMIT) -X github.com/storacha/piri/pkg/build.Date=$(DATE) -X github.com/storacha/piri/pkg/build.BuiltBy=make"
TAGS?=

.PHONY: all build piri install test test-verbose test-debug clean calibnet mockgen check-docs-links

all: build

build: piri

piri:
	go build $(GOFLAGS) $(TAGS) -o ./piri github.com/storacha/piri/cmd

install:
	go install ./cmd/storage

test:
	go test ./...

# Enhanced test target with better output formatting
test-verbose:
	@echo "🔍 Running tests with verbose output..."
	@go test -v -race ./... 2>&1 | tee test-output.log || (echo "❌ Tests failed. Check test-output.log for details" && exit 1)
	@echo "✅ All tests passed!"

# Test target that highlights failures for easier debugging
test-debug:
	@echo "🔍 Running tests with failure highlighting..."
	@if ! go test -v -race ./... 2>&1 | tee test-output.log; then \
		echo "\n🚨 Test failures detected:"; \
		grep -E "--- FAIL:|panic:|Error:|error:" test-output.log | head -20 || true; \
		echo "\nℹ️  Full output saved to test-output.log"; \
		exit 1; \
	fi
	@echo "✅ All tests passed!"

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


# special target that sets the calibnet tag and invokes build
calibnet: TAGS=-tags calibnet
calibnet: build

# Check for broken links in documentation
check-docs-links:
	@./scripts/check-docs-links.sh
