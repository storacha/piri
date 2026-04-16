VERSION=$(shell awk -F'"' '/"version":/ {print $$4}' version.json)
COMMIT=$(shell git rev-parse --short HEAD)
DATE=$(shell date -u -Iseconds)
GOFLAGS=-ldflags="-X github.com/storacha/piri/pkg/build.version=$(VERSION) -X github.com/storacha/piri/pkg/build.Commit=$(COMMIT) -X github.com/storacha/piri/pkg/build.Date=$(DATE) -X github.com/storacha/piri/pkg/build.BuiltBy=make"
TAGS?=

.PHONY: all build test clean

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

test:
	go test ./...

clean:
	rm -f ./piri