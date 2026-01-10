.PHONY: build clean test install

# Build variables
# Use the latest git tag if available, otherwise use 0.0.0-dev
# Strip the 'v' prefix from tags (v0.1.0 -> 0.1.0)
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null)
VERSION ?= $(if $(GIT_TAG),$(shell echo $(GIT_TAG) | sed 's/^v//'),0.0.0-dev)
LDFLAGS := -ldflags "-X github.com/clairitydev/cora/cmd.Version=$(VERSION)"
BINARY := cora

# Default target
all: build

# Build the binary
build:
	go build $(LDFLAGS) -o bin/$(BINARY) .

# Build for all platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64 .

# Install to GOPATH/bin
install:
	go install $(LDFLAGS) .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Development build and run
dev: build
	./bin/$(BINARY) --help
