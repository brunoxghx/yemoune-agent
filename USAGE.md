# Yemoune Agent - Usage Guide

## Quick Start: Send Scan to Yemoune Server

### 1. Configure the Agent

Create a configuration file (or use environment variables):

```bash
# Copy the example config
cp config.yaml.example config.yaml

# Edit the configuration
nano config.yaml
```

**Required settings:**
- `agent.server_url` - Your Yemoune server URL (e.g., `http://localhost:8080` or `https://yemoune.example.com`)
- `agent.api_token` - Authentication token (set via environment variable for security)

### 2. Set Environment Variables

**Recommended approach** - Set API token via environment variable:

```bash
export YEMOUNE_API_TOKEN="your-api-token-here"
export YEMOUNE_SERVER_URL="http://localhost:8080"  # Optional override
```

### 3. Run a Scan

**Basic scan:**
```bash
./dist/yemoune-agent-linux-amd64 scan /path/to/scan
```

**With custom configuration:**
```bash
./dist/yemoune-agent-linux-amd64 scan /path/to/scan --config /path/to/config.yaml
```

**With custom worker count:**
```bash
./dist/yemoune-agent-linux-amd64 scan /data --workers 20
```

**Verbose output:**
```bash
./dist/yemoune-agent-linux-amd64 scan /data -v
```

### 4. Test Without Server (Dry Run)

Test the scan without sending to server:

```bash
./dist/yemoune-agent-linux-amd64 scan /path/to/scan --dry-run
```

---

## Complete Example

Here's a complete workflow:

```bash
# 1. Set up environment
export YEMOUNE_API_TOKEN="my-secret-token-123"
export YEMOUNE_SERVER_URL="http://192.168.1.100:8080"

# 2. Create config file (optional - can use defaults)
cat > /etc/yemoune-agent/config.yaml <<EOF
agent:
  server_url: "${YEMOUNE_SERVER_URL}"
scanner:
  workers: 15
  exclude_paths:
    - "/proc/*"
    - "/sys/*"
    - "/dev/*"
    - "/tmp/*"
logging:
  level: "info"
  format: "json"
EOF

# 3. Test connectivity
./dist/yemoune-agent-linux-amd64 health

# 4. Validate configuration
./dist/yemoune-agent-linux-amd64 config validate

# 5. Run a dry-run test first
./dist/yemoune-agent-linux-amd64 scan /home --dry-run -v

# 6. Run actual scan (sends to server)
./dist/yemoune-agent-linux-amd64 scan /home
```

---

## Configuration Options

### Minimal Configuration (Environment Variables Only)

You can run without a config file using only environment variables:

```bash
export YEMOUNE_API_TOKEN="your-token"
export YEMOUNE_SERVER_URL="http://localhost:8080"

./dist/yemoune-agent-linux-amd64 scan /data
```

### Full Configuration File

See `config.yaml.example` for all available options.

---

## Server API Endpoint

The agent sends scan data to:
```
POST {server_url}/api/agent/report
```

**Request headers:**
- `Content-Type: application/json`
- `Content-Encoding: zstd`
- `Authorization: Bearer {api_token}`
- `X-Uncompressed-Size: {size_in_bytes}`

**Payload structure:**
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
      "permissions": 420,
      "modified_time": "2025-12-28T10:00:00Z",
      "accessed_time": "2025-12-28T11:00:00Z",
      "changed_time": "2025-12-28T10:00:00Z",
      "file_type": "regular",
      "extension": "txt"
    }
  ],
  "stats": {
    "files_scanned": 100,
    "directories_scanned": 10,
    "total_size": 1048576,
    "errors": 0
  }
}
```

**Expected response:**
- `200 OK` - Batch accepted
- `4xx` - Client error (invalid payload, authentication failed)
- `5xx` - Server error (will retry automatically)

---

## Batching Behavior

- Maximum batch size: **1GB uncompressed** (configurable via `scanner.batch_size_mb`)
- Compression: **zstd** (typically 3-5x compression ratio)
- Large scans will be split into multiple batches
- Each batch is sent as a separate HTTP POST
- The final batch includes `total_batches` field

**Example:**
- Scanning 5 million files (~1.3GB uncompressed)
- Results in 2 batches:
  - Batch 1: ~1GB (files 1-3.8M), `batch_number: 1`, `total_batches: null`
  - Batch 2: ~300MB (files 3.8M-5M), `batch_number: 2`, `total_batches: 2`

---

## Error Handling

The agent automatically handles errors:

1. **Network errors** - Retries with exponential backoff (max 3 retries)
2. **4xx errors** - Logs error and fails (no retry)
3. **5xx errors** - Retries with backoff
4. **Final failure** - Logs error and exits

**Failed batches:** Currently logged only. Future versions will save to disk for manual recovery.

---

## Performance Tuning

### Worker Count

Adjust based on your storage type:

```bash
# SSD/NVMe - Higher parallelism
./dist/yemoune-agent-linux-amd64 scan /data --workers 50

# HDD - Lower parallelism
./dist/yemoune-agent-linux-amd64 scan /data --workers 5

# Network storage (NFS, Ceph) - Moderate
./dist/yemoune-agent-linux-amd64 scan /mnt/nfs --workers 10
```

### Batch Size

For faster progress updates on large scans:

```yaml
scanner:
  batch_size_mb: 512  # Smaller batches = more frequent updates
```

### Compression Level

Trade CPU for bandwidth:

```yaml
reporter:
  compression_level: 1  # Faster, less compression (CPU-constrained)
  # or
  compression_level: 9  # Slower, better compression (network-constrained)
```

---

## Monitoring Progress

The agent outputs JSON logs showing progress:

```json
{"level":"info","msg":"Starting scan","path":"/data","workers":10}
{"level":"info","msg":"Sending batch","batch_number":1,"files_in_batch":100000}
{"level":"info","msg":"Payload sent successfully","status_code":200}
{"level":"info","msg":"Scan completed","files_scanned":1000000}
```

Monitor in real-time:
```bash
./dist/yemoune-agent-linux-amd64 scan /data 2>&1 | jq -r '.msg'
```

---

## Troubleshooting

### Check connectivity
```bash
./dist/yemoune-agent-linux-amd64 health
```

### Validate config
```bash
./dist/yemoune-agent-linux-amd64 config validate
```

### Show current config
```bash
./dist/yemoune-agent-linux-amd64 config show
```

### Verbose logging
```bash
./dist/yemoune-agent-linux-amd64 scan /data -v
```

### Test without sending
```bash
./dist/yemoune-agent-linux-amd64 scan /data --dry-run -v
```

---

## Security Best Practices

1. **Never hardcode tokens** in config files
   ```bash
   # Good
   export YEMOUNE_API_TOKEN="secret"

   # Bad
   echo "api_token: secret" >> config.yaml
   ```

2. **Use HTTPS** in production
   ```yaml
   agent:
     server_url: "https://yemoune.example.com"
   ```

3. **Restrict file permissions**
   ```bash
   chmod 600 /etc/yemoune-agent/config.yaml
   ```

4. **Run as dedicated user**
   ```bash
   useradd -r -s /bin/false yemoune-agent
   sudo -u yemoune-agent ./dist/yemoune-agent-linux-amd64 scan /data
   ```

---

## Example: Production Deployment

```bash
#!/bin/bash
# Production scan script

set -e

# Configuration
SCAN_PATH="/data"
CONFIG_FILE="/etc/yemoune-agent/config.yaml"
LOG_FILE="/var/log/yemoune-agent/scan-$(date +%Y%m%d-%H%M%S).log"

# Load environment
source /etc/yemoune-agent/agent.env

# Run scan
/usr/bin/yemoune-agent scan "$SCAN_PATH" \
  --config "$CONFIG_FILE" \
  2>&1 | tee "$LOG_FILE"

# Check exit status
if [ $? -eq 0 ]; then
    echo "Scan completed successfully"
else
    echo "Scan failed - check $LOG_FILE"
    exit 1
fi
```

---

## Next Steps

- [V1-Agent.md](V1-Agent.md) - Full design documentation
- [README.md](README.md) - Build instructions
- Set up systemd service for scheduled scans (coming soon)
