# Yemoune Agent Build Makefile
VERSION := 0.1.0-dev
BINARY_NAME := yemoune-agent
BUILD_DIR := ./dist
DOCKER_IMAGE := yemoune-agent-builder

# Output binaries
LINUX_BINARY := $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
MACOS_ARM_BINARY := $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64

.PHONY: all clean build-docker build-linux build-macos-arm build-all help

all: build-all

help:
	@echo "Yemoune Agent Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  make build-all         - Build for Linux and macOS ARM"
	@echo "  make build-linux       - Build for Linux amd64"
	@echo "  make build-macos-arm   - Build for macOS ARM (arm64)"
	@echo "  make build-docker      - Build Docker image for compilation"
	@echo "  make clean             - Remove build artifacts"
	@echo "  make test-local        - Test local Go build (no Docker)"
	@echo ""

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build Docker image
build-docker:
	@echo "Building Docker image for cross-compilation..."
	docker build -f Dockerfile.build -t $(DOCKER_IMAGE) .

# Build for Linux (amd64)
build-linux: $(BUILD_DIR) build-docker
	@echo "Building for Linux amd64..."
	docker run --rm \
		-v $(shell pwd):/build \
		-v $(shell pwd)/$(BUILD_DIR):/dist \
		-e GOOS=linux \
		-e GOARCH=amd64 \
		$(DOCKER_IMAGE) \
		go build -buildvcs=false -o /dist/$(BINARY_NAME)-linux-amd64 -ldflags "-X main.Version=$(VERSION)" ./cmd/yemoune-agent
	@echo "Linux binary: $(LINUX_BINARY)"

# Build for macOS ARM (M1/M2)
build-macos-arm: $(BUILD_DIR) build-docker
	@echo "Building for macOS ARM64..."
	docker run --rm \
		-v $(shell pwd):/build \
		-v $(shell pwd)/$(BUILD_DIR):/dist \
		-e GOOS=darwin \
		-e GOARCH=arm64 \
		$(DOCKER_IMAGE) \
		go build -buildvcs=false -o /dist/$(BINARY_NAME)-darwin-arm64 -ldflags "-X main.Version=$(VERSION)" ./cmd/yemoune-agent
	@echo "macOS ARM binary: $(MACOS_ARM_BINARY)"

# Build all platforms
build-all: build-linux build-macos-arm
	@echo ""
	@echo "Build complete! Binaries available in $(BUILD_DIR)/"
	@ls -lh $(BUILD_DIR)/

# Test local build without Docker (useful for quick testing)
test-local: $(BUILD_DIR)
	@echo "Building for current platform..."
	go build -buildvcs=false -o $(BUILD_DIR)/$(BINARY_NAME)-local -ldflags "-X main.Version=$(VERSION)" ./cmd/yemoune-agent
	@echo "Running local binary..."
	$(BUILD_DIR)/$(BINARY_NAME)-local

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	@echo "Clean complete!"

# Validate binaries (check if they exist and show file info)
validate:
	@echo "Validating built binaries..."
	@for binary in $(BUILD_DIR)/*; do \
		if [ -f "$$binary" ]; then \
			echo ""; \
			echo "Binary: $$binary"; \
			file $$binary; \
			ls -lh $$binary; \
		fi \
	done
