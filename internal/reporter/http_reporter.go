package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/brunoxghx/yemoune-agent/pkg/models"
	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
)

// HTTPReporter sends payloads to the server via HTTP
type HTTPReporter struct {
	serverURL         string
	apiToken          string
	client            *http.Client
	compressionLevel  int
	maxRetries        int
	retryDelaySeconds int
	logger            *zap.Logger
	encoder           *zstd.Encoder
}

// NewHTTPReporter creates a new HTTP reporter
func NewHTTPReporter(serverURL, apiToken string, timeoutSeconds, compressionLevel, maxRetries, retryDelaySeconds int, logger *zap.Logger) (*HTTPReporter, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}

	// Create zstd encoder
	var encoder *zstd.Encoder
	var err error
	if compressionLevel > 0 {
		encoder, err = zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(compressionLevel)))
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
		}
	}

	return &HTTPReporter{
		serverURL:         serverURL,
		apiToken:          apiToken,
		client:            client,
		compressionLevel:  compressionLevel,
		maxRetries:        maxRetries,
		retryDelaySeconds: retryDelaySeconds,
		logger:            logger,
		encoder:           encoder,
	}, nil
}

// SendPayload sends a payload to the server
func (r *HTTPReporter) SendPayload(ctx context.Context, payload *models.Payload) error {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	uncompressedSize := len(jsonData)

	// Compress payload
	var compressedData []byte
	if r.encoder != nil {
		compressedData = r.encoder.EncodeAll(jsonData, make([]byte, 0, len(jsonData)))
	} else {
		compressedData = jsonData
	}

	compressedSize := len(compressedData)
	compressionRatio := float64(uncompressedSize) / float64(compressedSize)

	r.logger.Info("Payload compressed",
		zap.Int("uncompressed_bytes", uncompressedSize),
		zap.Int("compressed_bytes", compressedSize),
		zap.Float64("compression_ratio", compressionRatio))

	// Send with retries
	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := time.Duration(r.retryDelaySeconds*(1<<(attempt-1))) * time.Second
			r.logger.Info("Retrying request",
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay))
			time.Sleep(delay)
		}

		err := r.sendRequest(ctx, compressedData, uncompressedSize)
		if err == nil {
			return nil
		}

		lastErr = err
		r.logger.Warn("Request failed",
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", r.maxRetries+1),
			zap.Error(err))
	}

	return fmt.Errorf("failed after %d attempts: %w", r.maxRetries+1, lastErr)
}

// sendRequest sends a single HTTP request
func (r *HTTPReporter) sendRequest(ctx context.Context, data []byte, uncompressedSize int) error {
	url := r.serverURL + "/api/agent/report"

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if r.encoder != nil {
		req.Header.Set("Content-Encoding", "zstd")
		req.Header.Set("X-Uncompressed-Size", fmt.Sprintf("%d", uncompressedSize))
	}
	if r.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiToken)
	}

	// Send request
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, _ := io.ReadAll(resp.Body)

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		r.logger.Info("Payload sent successfully",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return nil
	}

	// Handle error responses
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		// Client error - don't retry
		return fmt.Errorf("client error (status %d): %s", resp.StatusCode, string(body))
	}

	// Server error - will retry
	return fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(body))
}

// Close closes the reporter and releases resources
func (r *HTTPReporter) Close() error {
	if r.encoder != nil {
		r.encoder.Close()
	}
	return nil
}
