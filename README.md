# Yemoune Agent

High-performance, distributed file system scanner written in Go.

## Quick Start - Build Validation

This project uses Docker for cross-compilation to ensure consistent builds across Linux and macOS platforms.

### Prerequisites

- Docker installed and running
- Make utility

### Build Commands

```bash
# Build for all platforms (Linux + macOS ARM)
make build-all

# Build for specific platform
make build-linux       # Linux amd64
make build-macos-arm   # macOS ARM (M1/M2)

# Quick local test (builds for your current platform)
make test-local

# Clean build artifacts
make clean

# Show help
make help
```

### Output

All binaries are built into the `dist/` directory:
- `yemoune-agent-linux-amd64` - Linux binary
- `yemoune-agent-darwin-arm64` - macOS ARM binary

### Validate the Workflow

1. Build all binaries:
   ```bash
   make build-all
   ```

2. Validate the binaries:
   ```bash
   make validate
   ```

3. Test the Linux binary:
   ```bash
   ./dist/yemoune-agent-linux-amd64
   ```

4. Transfer macOS binary to a Mac and run:
   ```bash
   ./dist/yemoune-agent-darwin-arm64  # M1/M2 Mac
   ```

## Project Structure

```
yemoune-agent/
├── cmd/
│   └── yemoune-agent/
│       └── main.go           # Entry point
├── dist/                     # Built binaries (gitignored)
├── Dockerfile.build          # Docker build environment
├── Makefile                  # Build automation
├── go.mod                    # Go module definition
└── README.md                 # This file
```

## Documentation

See [V1-Agent.md](V1-Agent.md) for complete agent design and architecture.

## Development Status

Currently in validation phase - establishing build workflow and cross-compilation process.
