MODULE := github.com/jhuiting/chargebee-cli
BINARY := cb
BUILD_DIR := bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

.PHONY: build lint fmt verify test deps clean

build:
	@go build -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) ./cmd/cb

lint:
	@golangci-lint run

fmt:
	@go fmt ./...
	@goimports -w -local $(MODULE) .

verify: fmt lint build
	@echo "✅ Code verification complete"

test:
	@go test -v ./...

deps:
	@go mod download
	@go mod tidy

clean:
	@rm -rf $(BUILD_DIR)
