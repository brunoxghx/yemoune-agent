# Quick Start Guide

## Prerequisites

- Docker installed and running
- Bash shell (Linux/macOS/WSL)

## Build Commands

### Build Everything (Recommended)

```bash
cd /devGit/yemoune-agent
./build.sh all
```

This will create:
- `dist/yemoune-agent-linux-amd64` - Linux binary
- `dist/yemoune-agent-darwin-arm64` - macOS ARM (M1/M2) binary

### Build Specific Platform

```bash
./build.sh linux        # Linux only
./build.sh macos-arm    # macOS ARM only
```

### Test the Build

#### On Linux (current system)

```bash
./dist/yemoune-agent-linux-amd64
```

Expected output:
```
Yemoune Agent - Hello World
Version: 0.1.0-dev
OS: linux
Architecture: amd64
Go Version: go1.22.12

Workflow validation: SUCCESS
```

#### On macOS

Transfer the binary to your Mac:

```bash
# On ARM Mac (M1/M2)
./dist/yemoune-agent-darwin-arm64
```

### Validate Binaries

```bash
./build.sh validate
```

Shows file type and size for all built binaries.

### Clean Build Artifacts

```bash
./build.sh clean
```

Removes the `dist/` directory.

## Using Make (Alternative)

If you have `make` installed:

```bash
make build-all       # Build all platforms
make build-linux     # Build Linux only
make build-macos-arm # Build macOS ARM
make validate        # Validate binaries
make clean           # Clean artifacts
make help            # Show help
```

## Troubleshooting

### Docker not running

```
Error: Cannot connect to the Docker daemon
```

**Solution**: Start Docker daemon
```bash
sudo systemctl start docker  # Linux
# or open Docker Desktop on macOS/Windows
```

### Permission denied

```
Error: Permission denied while trying to connect to Docker
```

**Solution**: Add user to docker group or use sudo
```bash
sudo ./build.sh all
```

### Disk space

Build requires ~500MB for Docker image + ~10MB for binaries.

Check space:
```bash
docker system df
```

## Next Steps

1. ✅ Build workflow validated
2. 🔄 Ready for core implementation
3. 📝 See [V1-Agent.md](V1-Agent.md) for full design
4. 📋 See [VALIDATION.md](VALIDATION.md) for build report

## Help

```bash
./build.sh help
```
