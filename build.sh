#!/bin/bash

# Yemoune Agent Build Script
# Cross-compiles binaries for Linux and macOS using Docker

set -e

VERSION="0.1.0-dev"
BINARY_NAME="yemoune-agent"
BUILD_DIR="./dist"
DOCKER_IMAGE="yemoune-agent-builder"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Show help
show_help() {
    echo "Yemoune Agent Build System"
    echo ""
    echo "Usage: ./build.sh [command]"
    echo ""
    echo "Commands:"
    echo "  all              - Build for all platforms (default)"
    echo "  linux            - Build for Linux amd64"
    echo "  macos-arm        - Build for macOS ARM (arm64)"
    echo "  docker           - Build Docker image only"
    echo "  test-local       - Build and test locally (no Docker)"
    echo "  validate         - Validate built binaries"
    echo "  clean            - Remove build artifacts"
    echo "  help             - Show this help"
    echo ""
}

# Create build directory
create_build_dir() {
    if [ ! -d "$BUILD_DIR" ]; then
        mkdir -p "$BUILD_DIR"
        print_info "Created build directory: $BUILD_DIR"
    fi
}

# Build Docker image
build_docker_image() {
    print_info "Building Docker image for cross-compilation..."
    docker build -f Dockerfile.build -t "$DOCKER_IMAGE" .
    print_success "Docker image built: $DOCKER_IMAGE"
}

# Build for Linux
build_linux() {
    create_build_dir
    build_docker_image

    print_info "Building for Linux amd64..."
    docker run --rm \
        -v "$(pwd):/build" \
        -v "$(pwd)/$BUILD_DIR:/dist" \
        -e GOOS=linux \
        -e GOARCH=amd64 \
        "$DOCKER_IMAGE" \
        go build -buildvcs=false -o "/dist/${BINARY_NAME}-linux-amd64" \
        -ldflags "-X main.Version=$VERSION" \
        ./cmd/yemoune-agent

    print_success "Linux binary: $BUILD_DIR/${BINARY_NAME}-linux-amd64"
}

# Build for macOS ARM
build_macos_arm() {
    create_build_dir
    build_docker_image

    print_info "Building for macOS ARM64..."
    docker run --rm \
        -v "$(pwd):/build" \
        -v "$(pwd)/$BUILD_DIR:/dist" \
        -e GOOS=darwin \
        -e GOARCH=arm64 \
        "$DOCKER_IMAGE" \
        go build -buildvcs=false -o "/dist/${BINARY_NAME}-darwin-arm64" \
        -ldflags "-X main.Version=$VERSION" \
        ./cmd/yemoune-agent

    print_success "macOS ARM binary: $BUILD_DIR/${BINARY_NAME}-darwin-arm64"
}

# Build all platforms
build_all() {
    print_info "Building for all platforms..."
    echo ""

    build_linux
    echo ""

    build_macos_arm
    echo ""

    print_success "Build complete! Binaries available in $BUILD_DIR/"
    ls -lh "$BUILD_DIR/"
}

# Test local build
test_local() {
    create_build_dir

    print_info "Building for current platform..."
    go build -buildvcs=false -o "$BUILD_DIR/${BINARY_NAME}-local" \
        -ldflags "-X main.Version=$VERSION" \
        ./cmd/yemoune-agent

    print_info "Running local binary..."
    echo ""
    "$BUILD_DIR/${BINARY_NAME}-local"
    echo ""

    print_success "Local test completed!"
}

# Validate binaries
validate_binaries() {
    print_info "Validating built binaries..."
    echo ""

    for binary in "$BUILD_DIR"/*; do
        if [ -f "$binary" ]; then
            echo "Binary: $binary"
            file "$binary" 2>/dev/null || echo "file command not available"
            ls -lh "$binary"
            echo ""
        fi
    done

    print_success "Validation complete!"
}

# Clean build artifacts
clean() {
    print_info "Cleaning build artifacts..."
    rm -rf "$BUILD_DIR"
    print_success "Clean complete!"
}

# Main script
case "${1:-all}" in
    all)
        build_all
        ;;
    linux)
        build_linux
        ;;
    macos-arm)
        build_macos_arm
        ;;
    docker)
        build_docker_image
        ;;
    test-local)
        test_local
        ;;
    validate)
        validate_binaries
        ;;
    clean)
        clean
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
