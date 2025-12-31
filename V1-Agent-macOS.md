# Yemoune Agent - macOS Development Version

> **Project Status:** Planning Phase (Development Build)
> **Version:** 1.0-dev (macOS Development Support)
> **Last Updated:** December 27, 2025
> **Parent Document:** [V1-Agent.md](./V1-Agent.md)
> **Target:** macOS development environment (for testing before Linux production deployment)

---

## Overview

This document outlines the **macOS-specific adaptations** needed to develop and test the Yemoune Agent on **Apple Silicon Macs** before deploying to Linux production servers. The goal is to enable rapid development on macOS (ARM64) while maintaining compatibility with the Linux production target.

### Why macOS Development Build?

- **Developer Environment**: Apple Silicon Macs as primary development platform
- **Faster Iteration**: Build, test, and debug locally without VM/container overhead
- **Cross-Platform Go**: Go's excellent cross-platform support makes this straightforward
- **ARM64 Native**: Native ARM64 development for both macOS and Linux ARM64 servers
- **Pre-Production Testing**: Validate core logic before Linux deployment

---

## Key Differences: macOS vs Linux Production

| Feature | Linux Production | macOS Development | Notes |
|---------|------------------|-------------------|-------|
| **Filesystem ID** | `statfs.Fsid` (reliable) | `statfs.Fsid` (available but less reliable) | macOS Fsid changes on remount; acceptable for dev |
| **Filesystems** | ext4, XFS, Btrfs, NFS, Ceph | APFS, HFS+, NFS | Different FS types, same detection patterns |
| **Packaging** | RPM/DEB | Homebrew, DMG | Development: just binary; optional Homebrew formula |
| **Service Management** | systemd | launchd | Development: manual runs; optional launchd plist |
| **System Calls** | `unix.Statfs_t` (Linux) | `unix.Statfs_t` (Darwin) | Same package, different struct fields |
| **Paths** | `/etc/yemoune-agent/` | `~/.config/yemoune-agent/` or `/usr/local/etc/yemoune-agent/` | XDG-style for dev |
| **Permissions** | Service user `yemoune-agent` | Current user | No need for dedicated user in dev |

---

## Architecture Adaptations

### Filesystem ID Detection (Platform-Specific)

**Challenge**: macOS `statfs` returns different fields than Linux

**Solution**: Platform-specific implementations with build tags

```go
// internal/scanner/filesystem_darwin.go
//go:build darwin

package scanner

import (
    "fmt"
    "golang.org/x/sys/unix"
)

func getFilesystemID(path string) (string, error) {
    var stat unix.Statfs_t
    if err := unix.Statfs(path, &stat); err != nil {
        return "", fmt.Errorf("statfs failed: %w", err)
    }

    // macOS: Fsid is int32[2], format as hex
    // Note: May change on remount, but acceptable for dev/testing
    fsid := fmt.Sprintf("%x-%x", stat.Fsid.Val[0], stat.Fsid.Val[1])
    return fsid, nil
}

func getFilesystemType(path string) (string, error) {
    var stat unix.Statfs_t
    if err := unix.Statfs(path, &stat); err != nil {
        return "", fmt.Errorf("statfs failed: %w", err)
    }

    // macOS: Fstypename is [16]byte
    return unix.ByteSliceToString(stat.Fstypename[:]), nil
}
```

```go
// internal/scanner/filesystem_linux.go
//go:build linux

package scanner

import (
    "fmt"
    "golang.org/x/sys/unix"
)

func getFilesystemID(path string) (string, error) {
    var stat unix.Statfs_t
    if err := unix.Statfs(path, &stat); err != nil {
        return "", fmt.Errorf("statfs failed: %w", err)
    }

    // Linux: Fsid is struct{X__val [2]int32}
    fsid := fmt.Sprintf("%x-%x", stat.Fsid.X__val[0], stat.Fsid.X__val[1])
    return fsid, nil
}

func getFilesystemType(path string) (string, error) {
    var stat unix.Statfs_t
    if err := unix.Statfs(path, &stat); err != nil {
        return "", fmt.Errorf("statfs failed: %w", err)
    }

    // Linux: Type is int64, map to string
    return mapFsTypeToString(stat.Type), nil
}

func mapFsTypeToString(fsType int64) string {
    // Magic numbers from statfs(2)
    fsTypes := map[int64]string{
        0xEF53:     "ext4",
        0x58465342: "xfs",
        0x9123683E: "btrfs",
        0x6969:     "nfs",
        0x00C36400: "ceph",
        // ... more types
    }

    if name, ok := fsTypes[fsType]; ok {
        return name
    }
    return "unknown"
}
```

### Configuration Paths (macOS-Friendly)

**Development Configuration Locations**:

1. **User-specific** (recommended for dev): `~/.config/yemoune-agent/config.yaml`
2. **System-wide** (optional): `/usr/local/etc/yemoune-agent/config.yaml`
3. **Current directory** (for testing): `./config.yaml`

**Path Resolution Logic**:
```go
// internal/config/config.go
func getDefaultConfigPath() string {
    if runtime.GOOS == "darwin" {
        // macOS: Try user config first
        if home, err := os.UserHomeDir(); err == nil {
            userConfig := filepath.Join(home, ".config", "yemoune-agent", "config.yaml")
            if _, err := os.Stat(userConfig); err == nil {
                return userConfig
            }
        }
        // Fall back to system-wide
        return "/usr/local/etc/yemoune-agent/config.yaml"
    }

    // Linux: Use system config
    return "/etc/yemoune-agent/config.yaml"
}
```

### Logging (Console-Friendly for Development)

**Development Logging**:
- Default to console output (not file) on macOS
- Pretty-printed logs (not JSON) for easier reading
- Debug level enabled by default in dev mode

```go
// internal/logging/logger.go
func NewLogger(cfg LogConfig) (*zap.Logger, error) {
    var zapConfig zap.Config

    if runtime.GOOS == "darwin" && cfg.Environment == "development" {
        // macOS dev: Pretty console logs
        zapConfig = zap.NewDevelopmentConfig()
        zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    } else {
        // Linux production: JSON logs to file
        zapConfig = zap.NewProductionConfig()
        zapConfig.OutputPaths = []string{cfg.OutputPath}
    }

    return zapConfig.Build()
}
```

---

## Development Workflow

### Initial Setup on macOS

```bash
# 1. Clone repository
cd /devGit
git clone https://github.com/brunoxghx/yemoune-agent.git
cd yemoune-agent

# 2. Install Go dependencies
go mod download

# 3. Create config directory
mkdir -p ~/.config/yemoune-agent

# 4. Create development config
cat > ~/.config/yemoune-agent/config.yaml <<EOF
agent:
  id: "macos-dev"
  server_url: "http://localhost:8000"
  api_token: "${YEMOUNE_API_TOKEN}"

scanner:
  workers: 4  # Lower for dev
  batch_size_mb: 100  # Smaller batches for testing

  exclude_paths:
    - "/System/*"
    - "/Library/*"
    - "/Applications/*"
    - "*.app/*"
    - "*/node_modules/*"
    - "*/.git/*"

reporter:
  compression: "zstd"
  compression_level: 3
  timeout_seconds: 60
  max_retries: 3

logging:
  level: "debug"
  format: "console"
  output: "stdout"
EOF

# 5. Set API token (for local dev server)
export YEMOUNE_API_TOKEN="dev-token-12345"

# 6. Build binary
make build

# 7. Run a test scan
./bin/yemoune-agent scan ~/Documents --dry-run --verbose
```

### Build Commands (Apple Silicon)

```bash
# Build for Apple Silicon (native)
make build

# Build for Linux ARM64 (cross-compile for production ARM64 servers)
make build-linux-arm64

# Build for Linux AMD64 (cross-compile for production x86_64 servers)
make build-linux-amd64

# Build all Linux targets
make build-linux

# Run tests on macOS
make test

# Run with race detection
go test -race ./...

# Run locally without building
go run cmd/yemoune-agent/main.go scan /tmp

# Install to /opt/homebrew/bin (Apple Silicon default)
make install
```

### Makefile (Apple Silicon Optimized)

```makefile
# Makefile (Apple Silicon optimized)

BINARY_NAME=yemoune-agent
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Detect platform
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# Apple Silicon native build
build:
	@echo "Building for Apple Silicon (darwin/arm64)..."
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME} cmd/yemoune-agent/main.go

# Linux ARM64 (common for ARM-based servers: AWS Graviton, Ampere, etc.)
build-linux-arm64:
	@echo "Cross-compiling for Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-arm64 cmd/yemoune-agent/main.go

# Linux AMD64 (traditional x86_64 servers)
build-linux-amd64:
	@echo "Cross-compiling for Linux AMD64..."
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-amd64 cmd/yemoune-agent/main.go

# Build all Linux targets
build-linux: build-linux-arm64 build-linux-amd64

# Build everything
build-all: build build-linux

# Install to Homebrew path (Apple Silicon default: /opt/homebrew/bin)
install:
ifeq ($(UNAME_S),Darwin)
ifeq ($(UNAME_M),arm64)
	@echo "Installing to /opt/homebrew/bin..."
	go build ${LDFLAGS} -o /opt/homebrew/bin/${BINARY_NAME} cmd/yemoune-agent/main.go
	chmod +x /opt/homebrew/bin/${BINARY_NAME}
else
	@echo "Error: This build is optimized for Apple Silicon only"
	@exit 1
endif
else
	@echo "Error: This target is for macOS only"
	@exit 1
endif

# Test with coverage
test:
	@echo "Running tests with race detection..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Test coverage report
coverage:
	go tool cover -html=coverage.out

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Development run (no build)
run:
	go run cmd/yemoune-agent/main.go scan /tmp --dry-run --verbose

# Show build info
info:
	@echo "Platform: $(UNAME_S)/$(UNAME_M)"
	@echo "Go Version: $(shell go version)"
	@echo "Binary Name: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"

.PHONY: build build-linux-arm64 build-linux-amd64 build-linux build-all install test coverage clean run info
```

---

## Testing Strategy (macOS)

### Unit Tests (Platform-Agnostic)

Most unit tests will work identically on macOS and Linux:

```go
// tests/scanner_test.go
func TestFilesystemIDDetection(t *testing.T) {
    // Works on both platforms
    fsid, err := getFilesystemID("/tmp")
    require.NoError(t, err)
    assert.Regexp(t, "^[0-9a-f]+-[0-9a-f]+$", fsid)
}

func TestBatchManager(t *testing.T) {
    // Platform-agnostic: tests batch logic
    bm := NewBatchManager(100 * 1024 * 1024) // 100MB batches

    for i := 0; i < 10000; i++ {
        file := FileMetadata{
            Path: fmt.Sprintf("/test/file%d.txt", i),
            Size: 1024,
        }

        batch, shouldSend := bm.AddFile(file)
        if shouldSend {
            assert.NotNil(t, batch)
            // Send batch...
            bm.Reset()
        }
    }
}
```

### Platform-Specific Tests

Use build tags to test platform-specific code:

```go
// tests/scanner_darwin_test.go
//go:build darwin

package scanner_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestMacOSFilesystemTypes(t *testing.T) {
    tests := []struct {
        path     string
        expected string
    }{
        {"/", "apfs"},           // Root is usually APFS
        {"/System", "apfs"},     // System volume
        {"/tmp", "apfs"},        // Temp is on APFS
    }

    for _, tt := range tests {
        t.Run(tt.path, func(t *testing.T) {
            fsType, err := getFilesystemType(tt.path)
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, fsType)
        })
    }
}

func TestAPFSHardlinks(t *testing.T) {
    // APFS supports hardlinks, test detection
    tmpDir := t.TempDir()
    file1 := filepath.Join(tmpDir, "file1.txt")
    file2 := filepath.Join(tmpDir, "file2.txt")

    os.WriteFile(file1, []byte("test"), 0644)
    os.Link(file1, file2)

    meta1 := extractMetadata(file1)
    meta2 := extractMetadata(file2)

    assert.Equal(t, meta1.Inode, meta2.Inode)
    assert.Equal(t, uint32(2), meta1.LinkCount)
}
```

### Integration Testing with Local Backend

Run a local Yemoune backend for integration testing:

```bash
# Terminal 1: Start local backend (Docker)
cd /path/to/yemoune-backend
docker-compose up

# Terminal 2: Run agent against local backend
cd /devGit/yemoune-agent
export YEMOUNE_API_TOKEN="local-dev-token"
./bin/yemoune-agent scan ~/Documents --config configs/dev-local.yaml
```

---

## macOS-Specific Features (Optional)

### 1. Homebrew Formula (Apple Silicon)

For easier installation on Apple Silicon Macs:

```ruby
# packaging/homebrew/yemoune-agent.rb
class YemouneAgent < Formula
  desc "Distributed file system scanner for Yemoune"
  homepage "https://github.com/brunoxghx/yemoune"
  url "https://github.com/brunoxghx/yemoune-agent/archive/v1.0.0.tar.gz"
  sha256 "..."
  license "MIT"

  # Apple Silicon only
  depends_on arch: :arm64
  depends_on "go" => :build

  def install
    # Build native ARM64 binary
    system "make", "build"
    bin.install "bin/yemoune-agent"

    # Install example config to /opt/homebrew/etc
    (etc/"yemoune-agent").mkpath
    (etc/"yemoune-agent/config.yaml").write <<~EOS
      # Example configuration for Apple Silicon
      # See: https://github.com/brunoxghx/yemoune-agent
      agent:
        id: "#{`hostname`.strip}"
        server_url: "https://yemoune.example.com"

      scanner:
        workers: 4  # Optimized for Apple Silicon
        exclude_paths:
          - "/System/*"
          - "/Library/*"
          - "*.app/*"
    EOS
  end

  def caveats
    <<~EOS
      This formula is optimized for Apple Silicon (ARM64) only.

      Configuration file: /opt/homebrew/etc/yemoune-agent/config.yaml
      Or user config: ~/.config/yemoune-agent/config.yaml

      Set your API token:
        export YEMOUNE_API_TOKEN="your-token"

      Run a scan:
        yemoune-agent scan /path/to/directory
    EOS
  end

  test do
    assert_match "yemoune-agent version", shell_output("#{bin}/yemoune-agent version")
    # Verify it's ARM64
    assert_match "arm64", shell_output("file #{bin}/yemoune-agent")
  end
end
```

Installation:
```bash
# Add tap
brew tap brunoxghx/yemoune

# Install (Apple Silicon only)
brew install yemoune-agent

# Verify ARM64
file /opt/homebrew/bin/yemoune-agent
# Expected: Mach-O 64-bit executable arm64
```

### 2. Launchd Plist (Optional Scheduling)

For automated scans on macOS (equivalent to systemd timer):

```xml
<!-- packaging/launchd/com.yemoune.agent.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.yemoune.agent</string>

    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/yemoune-agent</string>
        <string>scan</string>
        <string>/Users</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
        <key>YEMOUNE_API_TOKEN</key>
        <string>your-token-here</string>
    </dict>

    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>2</integer>
        <key>Minute</key>
        <integer>0</integer>
    </dict>

    <key>StandardOutPath</key>
    <string>/usr/local/var/log/yemoune-agent/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>/usr/local/var/log/yemoune-agent/stderr.log</string>
</dict>
</plist>
```

Installation:
```bash
# Copy plist to user LaunchAgents
cp packaging/launchd/com.yemoune.agent.plist ~/Library/LaunchAgents/

# Edit token
vi ~/Library/LaunchAgents/com.yemoune.agent.plist

# Load and start
launchctl load ~/Library/LaunchAgents/com.yemoune.agent.plist
launchctl start com.yemoune.agent

# Check status
launchctl list | grep yemoune
```

### 3. macOS Filesystem Exclusions

Default exclusions for macOS-specific paths:

```yaml
# configs/config-darwin.yaml
scanner:
  exclude_paths:
    # System directories
    - "/System/*"
    - "/Library/*"
    - "/private/var/*"

    # Applications (binary bundles)
    - "/Applications/*"
    - "*.app/*"

    # macOS-specific caches
    - "*/Library/Caches/*"
    - "*/.Trash/*"
    - "*/.Spotlight-V100/*"
    - "*/.fseventsd/*"
    - "*/.DocumentRevisions-V100/*"

    # Development directories
    - "*/node_modules/*"
    - "*/.git/*"
    - "*/.svn/*"
    - "*/vendor/*"
    - "*/__pycache__/*"

    # IDE directories
    - "*/.idea/*"
    - "*/.vscode/*"
    - "*.xcworkspace/*"
```

---

## Development vs Production Differences

### Configuration Defaults

```go
// internal/config/defaults.go
func getDefaultConfig() Config {
    cfg := Config{
        Scanner: ScannerConfig{
            Workers:       10,
            MaxDepth:      0,
            BatchSizeMB:   1024,
        },
        Reporter: ReporterConfig{
            Compression:      "zstd",
            CompressionLevel: 3,
            TimeoutSeconds:   60,
            MaxRetries:       3,
        },
    }

    // Platform-specific adjustments
    if runtime.GOOS == "darwin" {
        // macOS: Lower defaults for dev
        cfg.Scanner.Workers = 4
        cfg.Scanner.BatchSizeMB = 100
        cfg.Logging.Level = "debug"
        cfg.Logging.Format = "console"
        cfg.Logging.Output = "stdout"
    }

    return cfg
}
```

### Performance Tuning

| Setting | Linux Production | macOS Development | Reason |
|---------|------------------|-------------------|--------|
| Workers | 10-50 | 4-8 | Fewer cores/lower I/O on dev machines |
| Batch Size | 1024 MB | 100-256 MB | Faster feedback, lower memory |
| Compression | Level 3 | Level 1 | Faster builds, less CPU |
| Log Level | info | debug | More verbose for debugging |
| Log Format | json | console | Easier to read during dev |

---

## Development Checklist

### Phase 1: Core Functionality (macOS)
- [ ] Setup Go project structure
- [ ] Implement platform-agnostic scanner core
- [ ] Implement Darwin-specific `filesystem_darwin.go`
- [ ] Implement Linux-specific `filesystem_linux.go`
- [ ] Implement batch manager
- [ ] Implement zstd compression
- [ ] Implement HTTP reporter
- [ ] Create CLI with Cobra
- [ ] Add platform detection and config path resolution

### Phase 2: Testing on macOS
- [ ] Write unit tests (platform-agnostic)
- [ ] Write platform-specific tests (`*_darwin_test.go`)
- [ ] Setup local backend for integration testing
- [ ] Test APFS hardlink detection
- [ ] Test NFS mount scanning (if available)
- [ ] Performance benchmarks on macOS

### Phase 3: Linux Validation
- [ ] Cross-compile for Linux ARM64 (primary target)
- [ ] Cross-compile for Linux AMD64 (secondary target)
- [ ] Test on Linux VM/container (ARM64 preferred)
- [ ] Validate ext4/XFS filesystem detection on Linux
- [ ] Test systemd service integration
- [ ] Benchmark performance on Linux ARM64 vs AMD64
- [ ] Create RPM/DEB packages for both architectures

### Phase 4: Optional macOS Enhancements
- [ ] Create Homebrew formula
- [ ] Create launchd plist
- [ ] Write macOS-specific documentation
- [ ] Add notarization for macOS distribution (if needed)

---

## Quick Start Commands

### Development on macOS

```bash
# Clone and setup
git clone https://github.com/brunoxghx/yemoune-agent.git
cd yemoune-agent
go mod download

# Create config
mkdir -p ~/.config/yemoune-agent
cp configs/config-darwin.yaml ~/.config/yemoune-agent/config.yaml

# Build
make build

# Test on small directory
./bin/yemoune-agent scan ~/Documents/test --dry-run --verbose

# Run tests
make test

# Cross-compile for Linux ARM64 (primary)
make build-linux-arm64

# Cross-compile for Linux AMD64
make build-linux-amd64

# Check version and architecture
./bin/yemoune-agent version
file ./bin/yemoune-agent  # Should show: Mach-O 64-bit executable arm64
```

### Testing Against Local Backend

```bash
# Terminal 1: Start backend
cd ../yemoune-backend
docker-compose up

# Terminal 2: Run agent
cd ../yemoune-agent
export YEMOUNE_API_TOKEN="dev-token"
./bin/yemoune-agent scan ~/test-data \
  --config ~/.config/yemoune-agent/config.yaml \
  --verbose
```

---

## Troubleshooting (macOS-Specific)

### Issue: Permission Denied on System Paths

**Problem**: Scanning `/Library` or `/System` requires elevated permissions

**Solution**:
```bash
# Option 1: Run with sudo
sudo ./bin/yemoune-agent scan /Library

# Option 2: Grant Full Disk Access
# System Preferences → Security & Privacy → Privacy → Full Disk Access
# Add Terminal.app or your IDE
```

### Issue: Filesystem ID Changes on Remount

**Problem**: APFS filesystem IDs can change on remount (rare, but possible)

**Solution**: This is acceptable for development. For production on Linux, this is not an issue.

### Issue: Slow Performance on External Drives

**Problem**: USB/Thunderbolt drives have high latency

**Solution**:
```yaml
scanner:
  workers: 2  # Reduce workers for external drives
```

### Issue: Homebrew Installation Conflicts

**Problem**: Multiple versions installed

**Solution**:
```bash
brew uninstall yemoune-agent
rm -rf /usr/local/bin/yemoune-agent
make install-darwin
```

---

## Cross-Platform Considerations

### Build Tags Summary

```go
// Use these build tags for platform-specific files:

// filesystem_darwin.go
//go:build darwin

// filesystem_linux.go
//go:build linux

// config_darwin.go
//go:build darwin

// Run platform-specific tests:
// go test -tags darwin ./...
// go test -tags linux ./...
```

### Conditional Compilation

```go
// Example: Platform-specific default paths
package config

import "runtime"

func GetDefaultConfigPath() string {
    switch runtime.GOOS {
    case "darwin":
        return getHomeDir() + "/.config/yemoune-agent/config.yaml"
    case "linux":
        return "/etc/yemoune-agent/config.yaml"
    default:
        return "./config.yaml"
    }
}
```

---

## Success Criteria (macOS Development)

### Must Have
- [ ] Builds successfully on Apple Silicon (darwin/arm64)
- [ ] Scans APFS filesystems correctly
- [ ] Detects hardlinks on APFS
- [ ] Compresses and sends batches to backend
- [ ] All unit tests pass on Apple Silicon macOS
- [ ] Cross-compiles to Linux ARM64 successfully
- [ ] Cross-compiles to Linux AMD64 successfully
- [ ] User-friendly config in `~/.config/`
- [ ] Installs to `/opt/homebrew/bin` (Apple Silicon Homebrew path)

### Nice to Have
- [ ] Homebrew formula for easy installation on Apple Silicon
- [ ] Launchd plist for scheduled scans
- [ ] macOS-specific documentation
- [ ] Performance comparable to Linux ARM64 servers (accounting for filesystem differences)
- [ ] Native ARM64 performance optimization

---

## References

- **Parent Plan**: [V1-Agent.md](./V1-Agent.md) - Linux production version
- **Go Cross-Compilation**: [https://go.dev/doc/install/source#environment](https://go.dev/doc/install/source#environment)
- **macOS statfs**: `man 2 statfs` on macOS
- **APFS Documentation**: [Apple File System Reference](https://developer.apple.com/documentation/foundation/file_system)
- **Homebrew Formula**: [https://docs.brew.sh/Formula-Cookbook](https://docs.brew.sh/Formula-Cookbook)
- **Launchd**: [https://www.launchd.info/](https://www.launchd.info/)

---

**Document Version:** 1.0 - macOS Development Support (Apple Silicon Only)
**Status:** Planning Phase
**Development Platform:** macOS 12+ on Apple Silicon (M1/M2/M3/M4)
**Production Targets:**
- Linux ARM64 (AWS Graviton, Ampere Altra, Arm Neoverse)
- Linux AMD64 (Traditional x86_64 servers)
