.PHONY: help build clean docker

# Variables
GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
	GOBIN = $(shell go env GOPATH)/bin
endif
BUILDER := $(GOBIN)/builder
# OCB version MUST match otelcol_version in builder-config.yaml
OCB_VERSION ?= v0.149.0

help:
	@echo "Collector Build Targets:"
	@echo "  make install-ocb - Install OpenTelemetry Collector Builder ($(OCB_VERSION))"
	@echo "  make build       - Build custom collector binary"
	@echo "  make docker      - Build Docker image"
	@echo "  make clean       - Clean build artifacts"

# Install OCB
install-ocb:
	@echo "Installing OCB $(OCB_VERSION) (go.opentelemetry.io/collector/cmd/builder)..."
	go install go.opentelemetry.io/collector/cmd/builder@$(OCB_VERSION)
	@echo "OCB installed successfully. You can now run: make build"

# Build custom collector
build:
	@echo "Building custom collector..."
	@test -f $(BUILDER) || (echo "builder not found at $(BUILDER). Run: make install-ocb from project root" && exit 1)
	GOWORK=off $(BUILDER) --config=builder-config.yaml
	@echo "Collector built successfully at ./build/custom-collector"

# Build collector Docker image
docker:
	@echo "Building Docker image..."
	docker build -t otel-custom-collector:latest ..
	@echo "Docker image built: otel-custom-collector:latest"

# Clean build artifacts
clean:
	@echo "Cleaning collector build artifacts..."
	rm -rf ./build
	@echo "Clean complete"

.PHONY: help install-ocb build docker clean
