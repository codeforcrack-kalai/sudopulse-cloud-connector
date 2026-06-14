BINARY  := sudopulse-connector
VERSION := 1.0.0
BUILD   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
OUTDIR  := build

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.buildDate=$(BUILD) \
	-X main.commit=$(COMMIT)

.PHONY: build build-all clean fmt lint test

## build: Build for the current platform
build:
	@echo "Building $(BINARY) for $$(go env GOOS)/$$(go env GOARCH)..."
	@mkdir -p $(OUTDIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTDIR)/$(BINARY) ./cmd/connector

## build-all: Cross-compile for all supported platforms
build-all:
	@mkdir -p $(OUTDIR)
	@echo "Building linux/amd64..."
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-amd64   ./cmd/connector
	@echo "Building linux/arm64..."
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-linux-arm64   ./cmd/connector
	@echo "Building darwin/amd64..."
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-darwin-amd64  ./cmd/connector
	@echo "Building darwin/arm64..."
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-darwin-arm64  ./cmd/connector
	@echo "Building windows/amd64..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUTDIR)/$(BINARY)-windows-amd64.exe ./cmd/connector
	@echo "Done."

## clean: Remove build artifacts
clean:
	rm -rf $(OUTDIR)

## fmt: Format all Go source files
fmt:
	go fmt ./...

## lint: Run go vet
lint:
	go vet ./...

## test: Run tests
test:
	go test -race -cover ./...
