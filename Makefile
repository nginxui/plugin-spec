# Contract toolchain. Run `make tools` once, then `make generate` after every
# change under proto/ and commit the results.

# buf finds protoc-gen-go and protoc-gen-go-grpc through PATH.
GOBIN := $(shell go env GOPATH)/bin
export PATH := $(GOBIN):$(PATH)

BUF ?= $(or $(shell command -v buf 2>/dev/null),$(GOBIN)/buf)
GEN_DIR := gen/go/nginxui/plugin/v1
METHODS := gen/methods.json

# Pinned so that regenerated files do not churn between machines.
BUF_VERSION := v1.73.0
PROTOC_GEN_GO_VERSION := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2

.PHONY: all tools generate lint check test

all: generate check

tools:
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

generate:
	rm -f $(GEN_DIR)/*.pb.go
	$(BUF) generate
	cd tools && go run ./methods -out ../$(METHODS)

lint:
	$(BUF) lint
	$(BUF) format --diff --exit-code

# check fails when the generated Go code or gen/methods.json is stale, then
# runs the tests of both Go modules.
check: lint
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
		$(BUF) generate -o "$$tmp" && \
		diff -r "$$tmp/$(GEN_DIR)" $(GEN_DIR) || \
		{ echo "$(GEN_DIR) is stale, run 'make generate'"; exit 1; }
	cd tools && go run ./methods -out ../$(METHODS) -check
	$(MAKE) test

test:
	cd gen/go && go vet ./... && go test ./...
	cd tools && go vet ./... && go test -count=1 ./...
