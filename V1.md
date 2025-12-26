# Yemoune Agent - Distributed File System Scanner

> **Project Status:** Planning & Design Phase
> **Version:** 1.0 (Agent-Specific Planning)
> **Last Updated:** December 26, 2025
> **Parent Project:** [Yemoune](https://github.com/brunoxghx/yemoune)

---

## Agent Overview

**Yemoune Agent** is a high-performance, distributed file system scanner written in Go. It is designed to be deployed globally across thousands of servers, scanning local and network file systems and reporting metadata to a centralized Yemoune backend.

### Key Design Principles

1. **Agent-Only Scanning**: The agent is the ONLY component that scans file systems. The backend NEVER scans.
2. **Lightweight & Fast**: Single binary, minimal dependencies, optimized for I/O performance
3. **Parallel Scanning**: Goroutines for efficient multi-threaded directory traversal
4. **Batch Reporting**: Sends compressed payloads (≤1GB uncompressed) to backend via HTTP POST
5. **Hardlink Detection**: Tracks filesystem ID + inode to deduplicate hardlinked files
6. **Cross-Platform Filesystem Support**: ext4, XFS, Btrfs, NFS, Ceph, SMB
7. **Independent Versioning**: SemVer versioning independent of backend/frontend

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    YEMOUNE AGENT                        │
│                                                         │
│  ┌─────────────┐      ┌──────────────┐                │
│  │   Scanner   │─────▶│ Batch Manager│                │
│  │  (Parallel) │      │              │                │
│  └─────────────┘      └──────┬───────┘                │
│         │                    │                         │
│         │                    ▼                         │
│         │            ┌──────────────┐                 │
│         │            │  Compressor  │                 │
│         │            │    (zstd)    │                 │
│         │            └──────┬───────┘                 │
│         │                    │                         │
│         ▼                    ▼                         │
│  ┌─────────────────────────────────┐                  │
│  │      HTTP Reporter              │                  │
│  │  POST /api/agent/report         │                  │
│  └─────────────┬───────────────────┘                  │
│                │                                       │
└────────────────┼───────────────────────────────────────┘
                 │ HTTP POST (zstd compressed ≤1GB)
                 │
                 ▼
         ┌───────────────┐
         │ Yemoune Server│
         │   (Backend)   │
         └───────────────┘
```

---

## Core Features

### 1. Parallel File System Scanning

**Goal**: Scan millions of files efficiently using goroutines

**Implementation**:
- Worker pool with configurable goroutine count (default: 10)
- Depth-first directory traversal
- Channel-based work distribution
- Graceful error handling (skip inaccessible files/directories)

**Metadata Extracted**:
- Path (absolute)
- Size (bytes)
- Inode number
- Link count (for hardlink detection)
- Filesystem ID (from statfs)
- Owner UID/GID
- Permissions (mode)
- Timestamps (modified, accessed, changed)
- File type (regular, directory, symlink)
- Extension (derived from path)

### 2. Hardlink Detection

**Goal**: Accurately identify hardlinked files to prevent double-counting disk space

**Strategy**:
- Use `statfs.Fsid` (filesystem UUID) + inode number
- Works with SAN/multipath, NFS, Ceph, XFS on SAN
- Track `link_count > 1` files
- Send both filesystem_id and inode to backend
- Backend performs deduplication during processing

**Why Filesystem ID?**:
- Device ID changes with multipath configurations
- Filesystem ID remains constant across mount points
- Works with shared filesystems (NFS, Ceph)

### 3. Batch Payload Management

**Goal**: Send scan results in manageable chunks to backend

**Constraints**:
- Maximum 1GB **uncompressed** per payload
- Compress with zstd before transmission (3-5x compression ratio)
- Multiple payloads per scan job for large directories

**Payload Structure**:
```json
{
  "scan_id": "uuid-v4",
  "agent_id": "hostname",
  "batch_number": 1,
  "total_batches": null,
  "filesystem_id": "8000000-1000000",
  "scan_path": "/data",
  "files": [
    {
      "path": "/data/file1.txt",
      "size": 1024,
      "inode": 12345,
      "link_count": 1,
      "filesystem_id": "8000000-1000000",
      "owner_uid": 1000,
      "owner_gid": 1000,
      "permissions": 0644,
      "modified_time": "2025-12-26T10:00:00Z",
      "accessed_time": "2025-12-26T11:00:00Z",
      "changed_time": "2025-12-26T10:00:00Z",
      "file_type": "regular",
      "extension": "txt"
    }
  ],
  "stats": {
    "files_scanned": 100000,
    "directories_scanned": 5000,
    "total_size": 10737418240,
    "errors": 0
  }
}
```

**Batch Logic**:
1. Accumulate file metadata in memory
2. Calculate uncompressed JSON size
3. When approaching 1GB limit, finalize batch
4. Compress with zstd
5. Send HTTP POST to backend
6. Reset buffer and continue scanning
7. Update `batch_number` for next batch
8. Set `total_batches` on final batch

### 4. Zstd Compression

**Goal**: Reduce network bandwidth and transmission time

**Implementation**:
- Library: `github.com/klauspost/compress/zstd`
- Compression level: Default (level 3, balanced speed/ratio)
- Target compression ratio: 3-5x
- Set HTTP headers:
  - `Content-Encoding: zstd`
  - `X-Uncompressed-Size: <bytes>`

**Example**:
- 1 million files = ~260MB uncompressed JSON
- Compressed: ~65MB (4x ratio)
- Transmission time: ~1-2 seconds on 1Gbps network

### 5. HTTP Reporter

**Goal**: Reliably send batches to backend with retries

**Features**:
- HTTP POST to `/api/agent/report`
- Automatic retries (3 attempts with exponential backoff)
- Timeout: 60 seconds per request
- Connection pooling for multiple batches
- TLS support for production deployments
- API token authentication (Bearer token)

**Error Handling**:
- Network errors: Retry with backoff
- 4xx errors: Log and fail (payload validation failed)
- 5xx errors: Retry with backoff
- Final failure: Save payload to disk for manual recovery

### 6. Configuration Management

**Goal**: Flexible configuration via YAML and environment variables

**Configuration File** (`/etc/yemoune-agent/config.yaml`):
```yaml
agent:
  id: "{{ hostname }}"  # Auto-populated
  server_url: "https://yemoune.example.com"
  api_token: "${YEMOUNE_API_TOKEN}"  # From environment

scanner:
  workers: 10  # Parallel goroutines
  max_depth: 0  # 0 = unlimited
  batch_size_mb: 1024  # 1GB uncompressed

  # Path exclusions (glob patterns)
  exclude_paths:
    - "/proc/*"
    - "/sys/*"
    - "/dev/*"
    - "*.tmp"

  # Size filters
  min_file_size: 0  # bytes
  max_file_size: 0  # 0 = unlimited

reporter:
  compression: "zstd"
  compression_level: 3  # Default
  timeout_seconds: 60
  max_retries: 3
  retry_delay_seconds: 5

logging:
  level: "info"  # debug, info, warn, error
  format: "json"  # json, text
  output: "/var/log/yemoune-agent/agent.log"
```

**Environment Variables**:
- `YEMOUNE_API_TOKEN`: API authentication token (required)
- `YEMOUNE_SERVER_URL`: Override server URL
- `YEMOUNE_LOG_LEVEL`: Override log level

### 7. CLI Interface

**Goal**: Simple command-line interface for manual and scheduled scans

**Commands**:

```bash
# Scan a directory
yemoune-agent scan /data

# Scan with custom config
yemoune-agent scan /data --config /etc/yemoune-agent/custom.yaml

# Test configuration
yemoune-agent config validate

# Show current configuration
yemoune-agent config show

# Check agent health
yemoune-agent health

# Show version
yemoune-agent version

# Register agent with server
yemoune-agent register --server https://yemoune.example.com --token <token>
```

**Flags**:
- `--config`: Path to config file (default: `/etc/yemoune-agent/config.yaml`)
- `--workers`: Override worker count
- `--dry-run`: Scan without sending to server
- `--verbose`: Enable debug logging

### 8. Systemd Integration

**Goal**: Run agent as a managed service with scheduling

**Service File** (`/etc/systemd/system/yemoune-agent.service`):
```ini
[Unit]
Description=Yemoune File System Scan Agent
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/bin/yemoune-agent scan /data
User=yemoune-agent
Group=yemoune-agent
EnvironmentFile=/etc/yemoune-agent/agent.env
StandardOutput=journal
StandardError=journal
SyslogIdentifier=yemoune-agent

[Install]
WantedBy=multi-user.target
```

**Timer File** (`/etc/systemd/system/yemoune-agent.timer`):
```ini
[Unit]
Description=Yemoune Agent Scan Timer
Requires=yemoune-agent.service

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

**Management**:
```bash
# Start scan manually
sudo systemctl start yemoune-agent

# Enable daily scans
sudo systemctl enable --now yemoune-agent.timer

# Check status
sudo systemctl status yemoune-agent.timer
sudo journalctl -u yemoune-agent -f
```

---

## Filesystem Support

### Supported Filesystems

| Filesystem | Support | Notes |
|------------|---------|-------|
| ext4       | ✅ Full | Native Linux filesystem |
| XFS        | ✅ Full | High-performance, SAN compatible |
| Btrfs      | ✅ Full | Modern Linux filesystem |
| NFS        | ✅ Full | Network filesystem, shared filesystem_id |
| Ceph       | ✅ Full | Distributed filesystem, native mount |
| SMB/CIFS   | ✅ Full | Windows shares via kernel mount |
| tmpfs      | ⚠️ Limited | Ephemeral, not recommended for scanning |
| procfs/sysfs | ❌ Excluded | Virtual filesystems, excluded by default |

### Filesystem ID Detection

**Implementation**:
```go
import "golang.org/x/sys/unix"

func getFilesystemID(path string) (string, error) {
    var stat unix.Statfs_t
    if err := unix.Statfs(path, &stat); err != nil {
        return "", fmt.Errorf("statfs failed: %w", err)
    }

    // Fsid is unique per filesystem
    // Works with SAN multipath, NFS, Ceph
    fsid := fmt.Sprintf("%x-%x", stat.Fsid.X__val[0], stat.Fsid.X__val[1])
    return fsid, nil
}
```

**Why This Works**:
- `statfs.Fsid` is the filesystem UUID
- Remains constant across mount points and device paths
- For NFS/Ceph: shared filesystems return the same fsid
- For SAN with multipath: same filesystem = same fsid regardless of device path

---

## Deployment

### Package Types

#### RPM Package (RHEL/CentOS/Rocky/AlmaLinux)

**Installation**:
```bash
# Install from repository
sudo yum install yemoune-agent

# Or install from file
sudo yum install yemoune-agent-1.0.0-1.el8.x86_64.rpm
```

**Package Contents**:
- Binary: `/usr/bin/yemoune-agent`
- Config: `/etc/yemoune-agent/config.yaml`
- Systemd service: `/etc/systemd/system/yemoune-agent.service`
- Systemd timer: `/etc/systemd/system/yemoune-agent.timer`
- Log directory: `/var/log/yemoune-agent/`
- User/group: `yemoune-agent:yemoune-agent`

#### DEB Package (Ubuntu/Debian)

**Installation**:
```bash
# Install from repository
sudo apt install yemoune-agent

# Or install from file
sudo dpkg -i yemoune-agent_1.0.0_amd64.deb
sudo apt-get install -f  # Fix dependencies
```

**Package Contents**: Same as RPM

### Post-Installation Setup

```bash
# 1. Configure API token
sudo vi /etc/yemoune-agent/agent.env
# Add: YEMOUNE_API_TOKEN=your-token-here

# 2. Configure scan paths
sudo vi /etc/yemoune-agent/config.yaml
# Update scan paths, exclusions, etc.

# 3. Test configuration
sudo yemoune-agent config validate

# 4. Test connectivity
sudo yemoune-agent health

# 5. Enable daily scans
sudo systemctl enable --now yemoune-agent.timer

# 6. Verify timer
sudo systemctl list-timers yemoune-agent
```

---

## Development

### Technology Stack

- **Language**: Go 1.22+
- **Configuration**: [Viper](https://github.com/spf13/viper) (YAML parsing)
- **Logging**: [Zap](https://github.com/uber-go/zap) (structured logging)
- **CLI**: [Cobra](https://github.com/spf13/cobra) (command-line interface)
- **Compression**: [klauspost/compress](https://github.com/klauspost/compress) (zstd)
- **HTTP Client**: Standard library `net/http`
- **Testing**: Standard library `testing` + [testify](https://github.com/stretchr/testify)

### Project Structure

```
yemoune-agent/
├── cmd/
│   └── yemoune-agent/
│       └── main.go           # Entry point
├── internal/
│   ├── scanner/
│   │   ├── scanner.go        # Core scanning logic
│   │   ├── worker_pool.go    # Parallel workers
│   │   └── filesystem.go     # Filesystem ID detection
│   ├── reporter/
│   │   ├── http_reporter.go  # HTTP client
│   │   └── batch_manager.go  # Batch payload logic
│   ├── config/
│   │   ├── config.go         # Configuration management
│   │   └── validator.go      # Config validation
│   └── cli/
│       ├── root.go           # Root command
│       ├── scan.go           # Scan command
│       ├── config.go         # Config commands
│       └── version.go        # Version command
├── pkg/
│   └── models/
│       ├── file_metadata.go  # File metadata struct
│       └── payload.go        # Payload struct
├── tests/
│   ├── scanner_test.go
│   ├── reporter_test.go
│   └── integration/
│       └── e2e_test.go
├── packaging/
│   ├── rpm/
│   │   ├── yemoune-agent.spec
│   │   └── systemd/
│   └── deb/
│       ├── control
│       └── systemd/
├── configs/
│   └── config.yaml.example
├── scripts/
│   ├── build.sh
│   ├── build-rpm.sh
│   └── build-deb.sh
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yml
├── V1.md                     # This document
└── README.md
```

### Build Commands

```bash
# Build binary
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run tests with coverage
make test-coverage

# Build RPM package
make rpm

# Build DEB package
make deb

# Build both packages
make packages

# Run locally
go run cmd/yemoune-agent/main.go scan /tmp

# Install locally
make install
```

### Testing Strategy

**Unit Tests** (>80% coverage target):
```go
// tests/scanner_test.go
func TestFilesystemIDDetection(t *testing.T) {
    fsid, err := getFilesystemID("/")
    require.NoError(t, err)
    assert.Regexp(t, "^[0-9a-f]+-[0-9a-f]+$", fsid)
}

func TestHardlinkDetection(t *testing.T) {
    // Create hardlink
    tmpDir := t.TempDir()
    file1 := filepath.Join(tmpDir, "file1.txt")
    file2 := filepath.Join(tmpDir, "file2.txt")

    os.WriteFile(file1, []byte("test"), 0644)
    os.Link(file1, file2)

    // Scan and verify
    metadata1 := extractMetadata(file1)
    metadata2 := extractMetadata(file2)

    assert.Equal(t, metadata1.Inode, metadata2.Inode)
    assert.Equal(t, metadata1.FilesystemID, metadata2.FilesystemID)
    assert.Equal(t, uint32(2), metadata1.LinkCount)
}
```

**Integration Tests**:
```go
// tests/integration/e2e_test.go
func TestFullScanWorkflow(t *testing.T) {
    // Setup mock server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "POST", r.Method)
        assert.Equal(t, "/api/agent/report", r.Path)
        assert.Equal(t, "zstd", r.Header.Get("Content-Encoding"))

        // Decompress and validate payload
        // ...

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
    }))
    defer server.Close()

    // Run scan
    scanner := NewScanner(Config{
        ServerURL: server.URL,
        Workers: 2,
    })

    err := scanner.Scan(t.TempDir())
    assert.NoError(t, err)
}
```

### CI/CD Pipeline

**GitHub Actions** (`.github/workflows/agent-ci.yml`):
```yaml
name: Agent CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run tests
        run: make test-coverage

      - name: Upload coverage
        uses: codecov/codecov-action@v3

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Build binary
        run: make build

      - name: Build packages
        run: make packages

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: packages
          path: |
            dist/*.rpm
            dist/*.deb
```

---

## Versioning & Releases

### Semantic Versioning

The agent follows [SemVer 2.0.0](https://semver.org/):

- **MAJOR**: Breaking changes to API contract or config format
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

**Examples**:
- `v1.0.0`: Initial release
- `v1.1.0`: Add new compression algorithm option
- `v1.1.1`: Fix hardlink detection bug
- `v2.0.0`: Change payload format (breaking change)

### Compatibility with Server

The agent maintains backward compatibility with the server API:

| Agent Version | Compatible Server Versions | Notes |
|---------------|---------------------------|-------|
| v1.0.x        | v1.0.x - v1.5.x          | Initial release |
| v1.1.x        | v1.0.x - v1.6.x          | Added config options |
| v2.0.x        | v1.5.x - v2.0.x          | New payload format |

**Breaking Changes Policy**:
- Server maintains backward compatibility for 2 minor versions
- Agent can upgrade independently of server
- Deprecated features removed after 6 months warning

### Release Process

1. **Development**: Feature branches → `develop` branch
2. **Testing**: CI/CD runs tests on every commit
3. **Release Candidate**: Tag `v1.2.0-rc1`, build packages
4. **Testing**: Deploy RC to staging environment
5. **Release**: Tag `v1.2.0`, build final packages
6. **Distribution**: Upload to package repositories
7. **Documentation**: Update changelog and compatibility matrix

---

## Security Considerations

### Authentication

- **API Token**: Bearer token for server authentication
- **Token Storage**: Environment variable, not in config file
- **Rotation**: Support token rotation without service restart

### Permissions

- **Service User**: Runs as `yemoune-agent:yemoune-agent`
- **File Access**: Requires read permissions on scanned directories
- **Recommended**: Run with minimal required permissions
- **Sudoers**: Optional sudoers entry for scanning restricted paths

### Network Security

- **TLS**: Support HTTPS for server communication
- **Certificate Validation**: Verify server certificates
- **Proxy Support**: HTTP/HTTPS proxy configuration
- **Firewall**: Outbound HTTPS (443) or custom port

### Data Privacy

- **No File Content**: Agent never reads file contents, only metadata
- **Path Filtering**: Exclude sensitive paths (e.g., `/home/*/.ssh`)
- **Compression**: Reduces data exposure during transmission

---

## Performance & Optimization

### Target Performance

- **Scan Speed**: 50,000+ files/second (SSD storage)
- **Memory Usage**: ~100MB base + ~5MB per 100K files buffered
- **CPU Usage**: ~200% (2 cores) with 10 workers
- **Network**: ~10-20 Mbps sustained during batch transmission

### Optimization Strategies

1. **Worker Pool Tuning**:
   - Default: 10 workers
   - SSD/NVMe: 20-50 workers
   - HDD/Network: 5-10 workers
   - Tune based on `iostat` and CPU usage

2. **Batch Size**:
   - Default: 1GB uncompressed
   - Large scans: Reduce to 500MB for more frequent progress updates
   - Small scans: Keep at 1GB to minimize HTTP overhead

3. **Compression Level**:
   - Default: Level 3 (balanced)
   - CPU-constrained: Level 1 (faster)
   - Network-constrained: Level 9 (better compression)

4. **Memory Management**:
   - Pre-allocate slices with estimated capacity
   - Reset batch buffer after transmission
   - Use sync.Pool for frequently allocated objects

---

## Success Criteria

### Functional Requirements

- [ ] Scan 1 million files without errors
- [ ] Correctly detect hardlinks across all supported filesystems
- [ ] Successfully send 1GB compressed payloads to server
- [ ] Handle network failures with retries
- [ ] Support all target filesystems (ext4, XFS, Btrfs, NFS, Ceph, SMB)
- [ ] Install and run as systemd service
- [ ] Package as RPM and DEB

### Performance Requirements

- [ ] Scan speed: 50,000+ files/second on SSD
- [ ] Memory usage: <200MB for 1M files
- [ ] CPU usage: <300% (3 cores) with 10 workers
- [ ] Compression ratio: 3x or better
- [ ] Network bandwidth: <50 Mbps sustained

### Quality Requirements

- [ ] Test coverage: >80%
- [ ] Zero critical security vulnerabilities
- [ ] No memory leaks (tested with 10M+ files)
- [ ] Graceful handling of permission errors
- [ ] Comprehensive error logging

---

## Roadmap

### Phase 1: Core Functionality (v1.0.0)
- [x] Planning and design
- [ ] Implement scanner with parallel workers
- [ ] Implement batch manager with 1GB limit
- [ ] Implement zstd compression
- [ ] Implement HTTP reporter with retries
- [ ] Implement CLI (scan, config, version)
- [ ] Implement filesystem ID detection
- [ ] Unit tests (>80% coverage)
- [ ] Integration tests

### Phase 2: Deployment (v1.1.0)
- [ ] RPM package creation
- [ ] DEB package creation
- [ ] Systemd service and timer
- [ ] Configuration management (Viper)
- [ ] Logging with structured output (Zap)
- [ ] Documentation (installation, configuration)
- [ ] CI/CD pipeline (GitHub Actions)

### Phase 3: Production Readiness (v1.2.0)
- [ ] TLS support
- [ ] Proxy support
- [ ] Token rotation
- [ ] Health check endpoint
- [ ] Metrics collection (Prometheus format)
- [ ] Performance benchmarks
- [ ] Security audit
- [ ] Package repository setup

### Phase 4: Advanced Features (v2.0.0+)
- [ ] Incremental scans (track changes since last scan)
- [ ] Real-time monitoring (inotify/fanotify)
- [ ] Plugin system for custom metadata extractors
- [ ] gRPC reporter (alternative to HTTP)
- [ ] Multi-server failover
- [ ] Advanced filtering (regex, size ranges)

---

## Appendix

### Glossary

- **Agent**: This program - scans file systems and reports to server
- **Scan**: Process of traversing a directory tree and extracting metadata
- **Batch**: Collection of file metadata sent as a single HTTP POST
- **Payload**: JSON-encoded batch data (compressed with zstd)
- **Filesystem ID**: Unique identifier from statfs (used for hardlink detection)
- **Hardlink**: Multiple directory entries pointing to the same inode
- **Worker**: Goroutine performing directory scanning
- **Compression Ratio**: Uncompressed size / compressed size

### References

- **Parent Project**: [Yemoune](https://github.com/brunoxghx/yemoune)
- **Go Documentation**: [https://go.dev/doc/](https://go.dev/doc/)
- **Zstd Compression**: [https://github.com/facebook/zstd](https://github.com/facebook/zstd)
- **Viper**: [https://github.com/spf13/viper](https://github.com/spf13/viper)
- **Cobra**: [https://github.com/spf13/cobra](https://github.com/spf13/cobra)
- **Zap**: [https://github.com/uber-go/zap](https://github.com/uber-go/zap)

---

**Document Version:** 1.0 - Agent Planning
**Status:** Planning Phase
**Repository:** brunoxghx/yemoune-agent
